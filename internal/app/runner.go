package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"nodes_check/internal/classifier"
	"nodes_check/internal/config"
	"nodes_check/internal/parser"
	"nodes_check/internal/precheck"
	iprobe "nodes_check/internal/probe"
	"nodes_check/internal/publisher"
	"nodes_check/internal/renderer"
	"nodes_check/internal/selector"
	"nodes_check/internal/storage"
	"nodes_check/internal/subscription"
)

type State struct {
	Running         bool           `json:"running"`
	Stage           string         `json:"stage"`
	Message         string         `json:"message"`
	LastError       string         `json:"last_error"`
	LastStartedAt   time.Time      `json:"last_started_at"`
	LastFinishedAt  time.Time      `json:"last_finished_at"`
	LastDuration    string         `json:"last_duration"`
	Counters        map[string]int `json:"counters"`
	CategoryCounts  map[string]int `json:"category_counts"`
	RecentLogs      []string       `json:"recent_logs"`
	LastOutputPath  string         `json:"last_output_path"`
	LastHistoryPath string         `json:"last_history_path"`
}

type Runner struct {
	configPath string

	mu     sync.RWMutex
	state  State
	cancel context.CancelFunc
}

type probeSummary struct {
	Total     int
	Success   int
	Failed    int
	Category  map[string]int
	BestDelay int64
}

type probeStageItem struct {
	candidate  precheck.Result
	aggregate  iprobe.AggregateResult
	failReason string
}

func NewRunner(configPath string) *Runner {
	return &Runner{
		configPath: configPath,
		state: State{
			Stage:          "idle",
			Message:        "waiting",
			Counters:       map[string]int{},
			CategoryCounts: map[string]int{},
			RecentLogs:     []string{},
		},
	}
}

func (r *Runner) State() State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := r.state
	s.Counters = cloneIntMap(s.Counters)
	s.CategoryCounts = cloneIntMap(s.CategoryCounts)
	s.RecentLogs = append([]string(nil), s.RecentLogs...)
	return s
}

func (r *Runner) StartScheduledLoop(ctx context.Context) error {
	cfg, err := config.Load(r.configPath)
	if err != nil {
		return err
	}
	if !cfg.Schedule.Enabled {
		r.logf("scheduler disabled")
		return nil
	}
	interval := time.Duration(cfg.Schedule.IntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = time.Hour
	}
	r.logf("scheduler enabled: every %s", interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.RunAsync(); err != nil {
					r.logf("scheduled run skipped: %v", err)
				}
			}
		}
	}()
	return nil
}

func (r *Runner) RunAsync() error {
	r.mu.Lock()
	if r.state.Running {
		r.mu.Unlock()
		return fmt.Errorf("job already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.state.Running = true
	r.state.Stage = "starting"
	r.state.Message = "job starting"
	r.state.LastError = ""
	r.state.LastStartedAt = time.Now()
	r.state.LastFinishedAt = time.Time{}
	r.state.LastDuration = ""
	r.state.Counters = map[string]int{}
	r.state.CategoryCounts = map[string]int{}
	r.state.RecentLogs = []string{}
	r.mu.Unlock()

	go func() {
		err := r.run(ctx)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.state.Running = false
		r.state.LastFinishedAt = time.Now()
		if !r.state.LastStartedAt.IsZero() {
			r.state.LastDuration = r.state.LastFinishedAt.Sub(r.state.LastStartedAt).Round(time.Second).String()
		}
		if err != nil {
			r.state.LastError = err.Error()
			r.state.Message = err.Error()
			if r.state.Stage == "done" {
				r.state.Stage = "failed"
			}
		} else {
			r.state.Stage = "done"
			r.state.Message = "job completed"
		}
		r.cancel = nil
	}()
	return nil
}

func (r *Runner) run(ctx context.Context) error {
	cfg, err := config.Load(r.configPath)
	if err != nil {
		return err
	}

	r.setStage("loading", "loading subscriptions")
	sources, err := subscription.LoadSourcesFromFile(cfg.Subscription.File)
	if err != nil {
		return err
	}
	r.setCounters(map[string]int{"sources": len(sources)})
	r.logf("config loaded: sources=%d output_dir=%s history_file=%s", len(sources), cfg.Output.Dir, cfg.Cache.HistoryFile)

	parsedNodes := make([]parser.Node, 0)
	fetchedCount := 0
	for _, source := range sources {
		fetched, err := subscription.Fetch(ctx, source.URL, cfg.ProbeTimeout())
		if err != nil {
			r.logf("subscription fetch failed: url=%s err=%v", source.URL, err)
			continue
		}
		fetchedCount++
		nodes, err := parser.ParseFetched(fetched)
		if err != nil {
			r.logf("subscription parse failed: url=%s err=%v", source.URL, err)
			continue
		}
		parsedNodes = append(parsedNodes, nodes...)
		r.logf("subscription fetched: url=%s parsed=%d", fetched.URL, len(nodes))
	}

	historyNodes, err := storage.LoadHistory(cfg.Cache.HistoryFile)
	if err != nil {
		r.logf("history load failed: file=%s err=%v", cfg.Cache.HistoryFile, err)
	} else if len(historyNodes) > 0 {
		parsedNodes = append(parsedNodes, historyNodes...)
		r.logf("history loaded: file=%s nodes=%d", cfg.Cache.HistoryFile, len(historyNodes))
	}
	if fetchedCount == 0 && len(historyNodes) == 0 {
		return fmt.Errorf("no subscriptions fetched successfully and no history nodes loaded")
	}

	parsedNodes = filterIPv4Nodes(parsedNodes)
	if len(parsedNodes) == 0 {
		return fmt.Errorf("no valid IPv4 nodes parsed from subscriptions or history")
	}
	r.setCounters(map[string]int{"parsed": len(parsedNodes), "history": len(historyNodes), "fetched_sources": fetchedCount})

	r.setStage("classifying", "classifying candidates")
	classifiedNodes := classifier.ClassifyAll(parsedNodes)
	dedupedNodes := precheck.Deduplicate(classifiedNodes)
	categoryCount := make(map[string]int)
	for _, node := range dedupedNodes {
		categoryCount[node.Category]++
	}
	r.setCategoryCounts(categoryCount)

	r.setStage("tcp_precheck", "running tcp precheck")
	prechecked := precheck.Run(ctx, dedupedNodes, cfg.TCPPrecheckTimeout(), cfg.Job.Concurrency)
	reachableCount := 0
	skippedCount := 0
	probeCandidates := make([]precheck.Result, 0, len(prechecked))
	for _, result := range prechecked {
		if result.TCPReachable {
			reachableCount++
			probeCandidates = append(probeCandidates, result)
		}
		if result.SkipReason != "" {
			skippedCount++
		}
	}
	probeCandidates = limitProbeCandidates(probeCandidates, cfg.Job.MaxProbeCandidates)
	r.setCounters(map[string]int{
		"parsed":           len(parsedNodes),
		"classified":       len(classifiedNodes),
		"deduped":          len(dedupedNodes),
		"tcp_pass":         reachableCount,
		"tcp_skip":         skippedCount,
		"probe_candidates": len(probeCandidates),
	})
	if len(probeCandidates) == 0 {
		return fmt.Errorf("no candidates passed tcp precheck")
	}

	probeCfg := iprobe.Config{
		ProbeURL:       cfg.Probe.ProbeURL,
		Timeout:        cfg.ProbeTimeout(),
		StartupTimeout: cfg.StartupTimeout(),
		Repeat:         cfg.Job.Repeat,
		Concurrency:    cfg.Job.Concurrency,
		XrayBin:        cfg.Probe.XrayBin,
		Template: iprobe.Template{
			Protocol:  cfg.Probe.Template.Protocol,
			UUID:      cfg.Probe.Template.UUID,
			Password:  cfg.Probe.Template.Password,
			Security:  cfg.Probe.Template.Security,
			Transport: cfg.Probe.Template.Transport,
			SNI:       cfg.Probe.Template.SNI,
			WSHost:    cfg.Probe.Template.WSHost,
			WSPath:    cfg.Probe.Template.WSPath,
		},
	}
	if err := iprobe.ValidateConfig(probeCfg); err != nil {
		return err
	}

	r.setStage("probe_phase1", "running probe phase 1")
	phase1Results, phase1Summary := r.runProbeStage(ctx, "phase1", probeCfg, probeCandidates, cfg.Probe.Phase1MaxDelayMS)
	r.setCounters(map[string]int{"phase1_pass": phase1Summary.Success, "phase1_fail": phase1Summary.Failed})

	phase2Candidates := make([]precheck.Result, 0, len(phase1Results))
	for _, item := range phase1Results {
		phase2Candidates = append(phase2Candidates, item.Precheck)
	}

	r.setStage("probe_phase2", "running probe phase 2")
	phase2Results, phase2Summary := r.runProbeStage(ctx, "phase2", probeCfg, phase2Candidates, cfg.Probe.Phase2MaxDelayMS)
	r.setCounters(map[string]int{
		"phase1_pass": phase1Summary.Success,
		"phase1_fail": phase1Summary.Failed,
		"phase2_pass": phase2Summary.Success,
		"phase2_fail": phase2Summary.Failed,
		"stable_pool": len(phase2Results),
	})

	r.setStage("history", "saving stable history pool")
	if len(phase2Results) == 0 {
		r.logf("history preserved: file=%s reason=empty stable pool", cfg.Cache.HistoryFile)
	} else if err := storage.SaveHistory(cfg.Cache.HistoryFile, phase2Results); err != nil {
		r.logf("history save failed: file=%s err=%v", cfg.Cache.HistoryFile, err)
	} else {
		r.mu.Lock()
		r.state.LastHistoryPath = cfg.Cache.HistoryFile
		r.mu.Unlock()
		r.logf("history saved: file=%s count=%d", cfg.Cache.HistoryFile, len(phase2Results))
	}

	r.setStage("selecting", "selecting final push sets")
	kvPlan := selector.NewPublishPlan(cfg.Publish.KV.Categories)
	dnsPlan := selector.NewPublishPlan(cfg.Publish.DNS.Categories)
	kvSelected := selector.Select(phase2Results, kvPlan, cfg.Output.NamePrefix)
	dnsSelected := selector.Select(phase2Results, dnsPlan, cfg.Output.NamePrefix)
	r.setCounters(map[string]int{
		"phase1_pass":  phase1Summary.Success,
		"phase2_pass":  phase2Summary.Success,
		"stable_pool":  len(phase2Results),
		"kv_selected":  len(kvSelected),
		"dns_selected": len(dnsSelected),
	})

	kvDomains := make(map[string]string, len(cfg.Cloudflare.DNS.Domains))
	for category, domain := range cfg.Cloudflare.DNS.Domains {
		kvDomains[category] = strings.TrimSpace(domain)
	}

	r.setStage("rendering", "writing kv output")
	outputs, err := renderer.WriteOutputs(cfg.Output.Dir, kvSelected, kvDomains)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.state.LastOutputPath = outputs.FinalIP
	r.mu.Unlock()
	r.logf("output written: final_ips=%s count=%d", outputs.FinalIP, len(kvSelected))

	finalContent, err := os.ReadFile(outputs.FinalIP)
	if err != nil {
		return err
	}

	r.setStage("publishing", "publishing kv and dns")
	publishResult := publisher.Publish(ctx, cfg, string(finalContent), dnsSelected)
	r.logf("publish kv: enabled=%t ok=%t message=%s", publishResult.KV.Enabled, publishResult.KV.OK, publishResult.KV.Message)
	for _, item := range publishResult.DNS {
		r.logf("publish dns: category=%s enabled=%t ok=%t domain=%s ip=%s message=%s", item.Category, item.Enabled, item.OK, item.Domain, item.IP, item.Message)
	}

	return nil
}

func (r *Runner) runProbeStage(ctx context.Context, stage string, cfg iprobe.Config, candidates []precheck.Result, maxDelayMS int64) ([]selector.Candidate, probeSummary) {
	_ = ctx
	summary := probeSummary{Total: len(candidates), Category: make(map[string]int)}
	selectedCandidates := make([]selector.Candidate, 0, len(candidates))

	results := make(chan probeStageItem, len(candidates))
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for _, candidate := range candidates {
		candidate := candidate
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			aggregate := iprobe.RunCandidate(cfg, candidate.Node.Node)
			failReason := "unknown"
			if len(aggregate.Results) > 0 && aggregate.Results[0].FailReason != "" {
				failReason = aggregate.Results[0].FailReason
			}
			results <- probeStageItem{candidate: candidate, aggregate: aggregate, failReason: failReason}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for item := range results {
		candidate := item.candidate
		aggregate := item.aggregate
		delay := aggregate.Summary.MinDelayMS
		if aggregate.Summary.SuccessCount > 0 && delay > 0 && delay <= maxDelayMS {
			summary.Success++
			summary.Category[candidate.Category]++
			selectedCandidates = append(selectedCandidates, selector.Candidate{Precheck: candidate, Aggregate: aggregate})
			if summary.BestDelay == 0 || delay < summary.BestDelay {
				summary.BestDelay = delay
			}
			r.logf("%s ok: category=%s name=%s host=%s port=%d delay_ms=%d", stage, candidate.Category, candidate.Name, candidate.Host, candidate.Port, delay)
			continue
		}
		summary.Failed++
		reason := item.failReason
		if aggregate.Summary.SuccessCount > 0 && delay > maxDelayMS {
			reason = fmt.Sprintf("delay_exceeded_%dms", maxDelayMS)
		}
		r.logf("%s fail: category=%s name=%s host=%s port=%d reason=%s", stage, candidate.Category, candidate.Name, candidate.Host, candidate.Port, reason)
	}

	sort.Slice(selectedCandidates, func(i, j int) bool {
		if selectedCandidates[i].Precheck.Category != selectedCandidates[j].Precheck.Category {
			return selectedCandidates[i].Precheck.Category < selectedCandidates[j].Precheck.Category
		}
		return selectedCandidates[i].Aggregate.Summary.MinDelayMS < selectedCandidates[j].Aggregate.Summary.MinDelayMS
	})
	return selectedCandidates, summary
}

func (r *Runner) setStage(stage, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.Stage = stage
	r.state.Message = message
}

func (r *Runner) setCounters(counters map[string]int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state.Counters == nil {
		r.state.Counters = map[string]int{}
	}
	for k, v := range counters {
		r.state.Counters[k] = v
	}
}

func (r *Runner) setCategoryCounts(counts map[string]int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.CategoryCounts = cloneIntMap(counts)
}

func (r *Runner) logf(format string, args ...any) {
	line := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.RecentLogs = append(r.state.RecentLogs, line)
	if len(r.state.RecentLogs) > 200 {
		r.state.RecentLogs = append([]string(nil), r.state.RecentLogs[len(r.state.RecentLogs)-200:]...)
	}
	r.state.Message = strings.TrimSpace(fmt.Sprintf(format, args...))
}

func limitProbeCandidates(items []precheck.Result, max int) []precheck.Result {
	if max <= 0 || len(items) <= max {
		return items
	}
	grouped := make(map[string][]precheck.Result)
	categories := make([]string, 0)
	for _, item := range items {
		if _, ok := grouped[item.Category]; !ok {
			categories = append(categories, item.Category)
		}
		grouped[item.Category] = append(grouped[item.Category], item)
	}
	sort.Strings(categories)
	for category := range grouped {
		bucket := grouped[category]
		sort.Slice(bucket, func(i, j int) bool {
			if bucket[i].TCPDelayMS != bucket[j].TCPDelayMS {
				return bucket[i].TCPDelayMS < bucket[j].TCPDelayMS
			}
			return bucket[i].Name < bucket[j].Name
		})
		grouped[category] = bucket
	}
	limited := make([]precheck.Result, 0, max)
	for len(limited) < max {
		progressed := false
		for _, category := range categories {
			bucket := grouped[category]
			if len(bucket) == 0 {
				continue
			}
			limited = append(limited, bucket[0])
			grouped[category] = bucket[1:]
			progressed = true
			if len(limited) >= max {
				break
			}
		}
		if !progressed {
			break
		}
	}
	return limited
}

func filterIPv4Nodes(nodes []parser.Node) []parser.Node {
	filtered := make([]parser.Node, 0, len(nodes))
	for _, node := range nodes {
		ip := net.ParseIP(node.Host)
		if ip == nil || ip.To4() == nil {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}

func cloneIntMap(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
