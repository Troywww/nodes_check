package probe

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"nodes_check/internal/parser"

	"golang.org/x/net/proxy"
)

type Node struct {
	RawURL    string `json:"-"`
	Name      string `json:"node_name"`
	Protocol  string `json:"protocol"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	UUID      string `json:"-"`
	Password  string `json:"-"`
	Security  string `json:"-"`
	Transport string `json:"-"`
	SNI       string `json:"-"`
	WSHost    string `json:"-"`
	WSPath    string `json:"-"`
}

type Template struct {
	Protocol  string
	UUID      string
	Password  string
	Security  string
	Transport string
	SNI       string
	WSHost    string
	WSPath    string
}

type SingleResult struct {
	NodeName     string `json:"node_name"`
	Protocol     string `json:"protocol"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ProbeURL     string `json:"probe_url"`
	Success      bool   `json:"success"`
	ProxyReady   bool   `json:"proxy_ready"`
	StatusCode   int    `json:"status_code"`
	RealDelayMS  int64  `json:"real_delay_ms"`
	FailReason   string `json:"fail_reason"`
	StartupMS    int64  `json:"startup_ms,omitempty"`
	ProxyReadyMS int64  `json:"proxy_ready_ms,omitempty"`
}

type Summary struct {
	Repeat       int   `json:"repeat"`
	SuccessCount int   `json:"success_count"`
	FailCount    int   `json:"fail_count"`
	AvgDelayMS   int64 `json:"avg_delay_ms"`
	MinDelayMS   int64 `json:"min_delay_ms"`
	MaxDelayMS   int64 `json:"max_delay_ms"`
}

type AggregateResult struct {
	Summary Summary        `json:"summary"`
	Results []SingleResult `json:"results"`
}

type BatchItem struct {
	Index      int             `json:"index"`
	Raw        string          `json:"raw"`
	OK         bool            `json:"ok"`
	BestResult *SingleResult   `json:"best_result,omitempty"`
	Result     AggregateResult `json:"result"`
}

type BatchOutput struct {
	File        string      `json:"file"`
	ProbeURL    string      `json:"probe_url"`
	Repeat      int         `json:"repeat"`
	Concurrency int         `json:"concurrency"`
	Items       []BatchItem `json:"items"`
}

type Config struct {
	ProbeURL       string
	Timeout        time.Duration
	StartupTimeout time.Duration
	Repeat         int
	Concurrency    int
	XrayBin        string
	Verbose        bool
	Template       Template
}

func ValidateConfig(cfg Config) error {
	if cfg.Repeat < 1 {
		return errors.New("repeat must be >= 1")
	}
	if cfg.Concurrency < 1 {
		return errors.New("concurrency must be >= 1")
	}
	if cfg.XrayBin == "" {
		return errors.New("xray binary is required")
	}
	if _, err := os.Stat(cfg.XrayBin); err == nil {
		return nil
	}
	if _, err := exec.LookPath(cfg.XrayBin); err == nil {
		return nil
	}
	return fmt.Errorf("missing xray binary: %s", cfg.XrayBin)
}

func RunOne(cfg Config, raw string) AggregateResult {
	return runAggregate(cfg, func() (Node, error) { return parseNode(raw) })
}

func RunCandidate(cfg Config, candidate parser.Node) AggregateResult {
	return runAggregate(cfg, func() (Node, error) { return buildNodeFromTemplate(cfg.Template, candidate) })
}

func RunBatch(cfg Config, file string) (BatchOutput, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return BatchOutput{}, err
	}

	type job struct {
		Index int
		Raw   string
	}

	lines := strings.Split(string(content), "\n")
	jobs := make([]job, 0, len(lines))
	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		jobs = append(jobs, job{Index: idx, Raw: line})
	}

	results := make([]BatchItem, 0, len(jobs))
	resultCh := make(chan BatchItem, len(jobs))
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for _, currentJob := range jobs {
		currentJob := currentJob
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			aggregate := runAggregate(cfg, func() (Node, error) { return parseNode(currentJob.Raw) })
			resultCh <- BatchItem{Index: currentJob.Index, Raw: currentJob.Raw, OK: aggregate.Summary.SuccessCount > 0, BestResult: pickBestResult(aggregate), Result: aggregate}
		}()
	}

	go func() { wg.Wait(); close(resultCh) }()
	for item := range resultCh {
		results = append(results, item)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Index < results[j].Index })

	return BatchOutput{File: file, ProbeURL: cfg.ProbeURL, Repeat: cfg.Repeat, Concurrency: cfg.Concurrency, Items: results}, nil
}

func runAggregate(cfg Config, resolve func() (Node, error)) AggregateResult {
	results := make([]SingleResult, 0, cfg.Repeat)
	successCount := 0
	var total int64
	var min int64
	var max int64

	for i := 0; i < cfg.Repeat; i++ {
		result := probeOnce(cfg, resolve)
		results = append(results, result)
		if result.Success {
			successCount++
			total += result.RealDelayMS
			if min == 0 || result.RealDelayMS < min {
				min = result.RealDelayMS
			}
			if result.RealDelayMS > max {
				max = result.RealDelayMS
			}
		}
	}

	summary := Summary{Repeat: cfg.Repeat, SuccessCount: successCount, FailCount: cfg.Repeat - successCount, MinDelayMS: min, MaxDelayMS: max}
	if successCount > 0 {
		summary.AvgDelayMS = total / int64(successCount)
	}
	return AggregateResult{Summary: summary, Results: results}
}

func pickBestResult(aggregate AggregateResult) *SingleResult {
	var best *SingleResult
	for i := range aggregate.Results {
		item := aggregate.Results[i]
		if !item.Success {
			continue
		}
		if best == nil || item.RealDelayMS < best.RealDelayMS {
			copyItem := item
			best = &copyItem
		}
	}
	return best
}

func probeOnce(cfg Config, resolve func() (Node, error)) SingleResult {
	node, err := resolve()
	if err != nil {
		return SingleResult{ProbeURL: cfg.ProbeURL, Success: false, FailReason: "parse_failed"}
	}
	if err := validateNode(node); err != nil {
		return SingleResult{NodeName: node.Name, Protocol: node.Protocol, Host: node.Host, Port: node.Port, ProbeURL: cfg.ProbeURL, Success: false, FailReason: err.Error()}
	}

	result := SingleResult{NodeName: node.Name, Protocol: node.Protocol, Host: node.Host, Port: node.Port, ProbeURL: cfg.ProbeURL}
	tmpDir, err := os.MkdirTemp("", "xray-probe-*")
	if err != nil {
		result.FailReason = "temp_dir_failed"
		return result
	}
	defer os.RemoveAll(tmpDir)

	localPort, err := randomPort()
	if err != nil {
		result.FailReason = "random_port_failed"
		return result
	}

	configPath := filepath.Join(tmpDir, "config.json")
	if err := writeXrayConfig(configPath, node, localPort); err != nil {
		result.FailReason = "config_build_failed"
		return result
	}

	logf(cfg.Verbose, "testing %s via %s %s:%d", node.Name, node.Protocol, node.Host, node.Port)
	cmd := exec.Command(cfg.XrayBin, "run", "-c", configPath)
	logFile, _ := os.Create(filepath.Join(tmpDir, "xray.log"))
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		result.FailReason = "xray_start_failed"
		return result
	}
	defer stopProcess(cmd)

	startupStart := time.Now()
	readyDuration, err := waitForProxy(cmd, localPort, cfg.StartupTimeout)
	result.StartupMS = time.Since(startupStart).Milliseconds()
	if err != nil {
		result.FailReason = err.Error()
		if logText, readErr := tailLog(filepath.Join(tmpDir, "xray.log")); readErr == nil && logText != "" {
			logf(cfg.Verbose, "xray log: %s", logText)
		}
		if result.FailReason == "proxy_not_ready" {
			time.Sleep(300 * time.Millisecond)
			readyDuration, err = waitForProxy(cmd, localPort, 2*time.Second)
			if err == nil {
				result.FailReason = ""
				result.ProxyReady = true
				result.ProxyReadyMS = readyDuration.Milliseconds()
			}
		}
		if !result.ProxyReady {
			return result
		}
	} else {
		result.ProxyReady = true
		result.ProxyReadyMS = readyDuration.Milliseconds()
	}

	reqStart := time.Now()
	statusCode, err := probeURL(cfg.ProbeURL, localPort, cfg.Timeout)
	result.RealDelayMS = time.Since(reqStart).Milliseconds()
	result.StatusCode = statusCode
	if err != nil {
		result.FailReason = "request_failed"
		return result
	}
	if statusCode == http.StatusNoContent || statusCode == http.StatusOK {
		result.Success = true
		return result
	}
	result.FailReason = "unexpected_status_code"
	return result
}

func buildNodeFromTemplate(template Template, candidate parser.Node) (Node, error) {
	node := Node{
		Name:      candidate.Name,
		Host:      candidate.Host,
		Port:      candidate.Port,
		Protocol:  strings.ToLower(template.Protocol),
		UUID:      template.UUID,
		Password:  template.Password,
		Security:  strings.ToLower(template.Security),
		Transport: strings.ToLower(template.Transport),
		SNI:       template.SNI,
		WSHost:    template.WSHost,
		WSPath:    template.WSPath,
	}
	if node.Name == "" {
		node.Name = "unnamed"
	}
	if node.Security == "" {
		node.Security = "tls"
	}
	if node.WSPath == "" {
		node.WSPath = "/"
	}
	if !strings.HasPrefix(node.WSPath, "/") {
		node.WSPath = "/" + node.WSPath
	}
	if node.SNI == "" {
		node.SNI = node.Host
	}
	if node.WSHost == "" {
		node.WSHost = node.SNI
	}
	return node, nil
}

func parseNode(raw string) (Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Node{}, err
	}

	node := Node{RawURL: raw, Protocol: strings.ToLower(u.Scheme), Name: decodeName(u.Fragment), Security: strings.ToLower(u.Query().Get("security")), Transport: strings.ToLower(u.Query().Get("type")), SNI: u.Query().Get("sni"), WSHost: u.Query().Get("host"), WSPath: u.Query().Get("path")}
	if node.Name == "" {
		node.Name = "unnamed"
	}
	if node.WSPath == "" {
		node.WSPath = "/"
	}
	if !strings.HasPrefix(node.WSPath, "/") {
		node.WSPath = "/" + node.WSPath
	}
	if node.Security == "" {
		node.Security = "tls"
	}

	host := u.Hostname()
	portStr := u.Port()
	if host == "" || portStr == "" {
		return Node{}, errors.New("missing host or port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Node{}, err
	}
	node.Host = host
	node.Port = port

	switch node.Protocol {
	case "vless":
		node.UUID = u.User.Username()
		if node.Transport == "" {
			node.Transport = "ws"
		}
	case "trojan":
		node.Password = u.User.Username()
		if node.Transport == "" {
			node.Transport = "tcp"
		}
	default:
		return node, errors.New("unsupported_protocol")
	}

	if node.SNI == "" {
		node.SNI = node.WSHost
	}
	if node.WSHost == "" {
		node.WSHost = node.SNI
	}
	if node.SNI == "" {
		node.SNI = node.Host
	}
	if node.WSHost == "" {
		node.WSHost = node.Host
	}
	return node, nil
}

func decodeName(fragment string) string {
	if fragment == "" {
		return ""
	}
	name, err := url.QueryUnescape(fragment)
	if err != nil {
		return fragment
	}
	return name
}

func validateNode(node Node) error {
	switch node.Protocol {
	case "vless":
		if node.UUID == "" {
			return errors.New("invalid_node")
		}
		if node.Security != "tls" {
			return errors.New("unsupported_security")
		}
		if node.Transport != "ws" {
			return errors.New("unsupported_transport")
		}
	case "trojan":
		if node.Password == "" {
			return errors.New("invalid_node")
		}
		if node.Security != "tls" {
			return errors.New("unsupported_security")
		}
		if node.Transport != "tcp" && node.Transport != "ws" {
			return errors.New("unsupported_transport")
		}
	default:
		return errors.New("unsupported_protocol")
	}
	return nil
}

func writeXrayConfig(path string, node Node, localPort int) error {
	type inbound struct {
		Tag      string                 `json:"tag"`
		Listen   string                 `json:"listen"`
		Port     int                    `json:"port"`
		Protocol string                 `json:"protocol"`
		Settings map[string]interface{} `json:"settings"`
	}
	type outbound struct {
		Tag            string                 `json:"tag"`
		Protocol       string                 `json:"protocol"`
		Settings       map[string]interface{} `json:"settings"`
		StreamSettings map[string]interface{} `json:"streamSettings,omitempty"`
	}

	cfg := map[string]interface{}{"log": map[string]string{"loglevel": "warning"}, "inbounds": []inbound{{Tag: "socks-in", Listen: "127.0.0.1", Port: localPort, Protocol: "socks", Settings: map[string]interface{}{"udp": false}}}}

	var ob outbound
	switch node.Protocol + ":" + node.Transport {
	case "vless:ws":
		ob = outbound{Tag: "proxy", Protocol: "vless", Settings: map[string]interface{}{"vnext": []map[string]interface{}{{"address": node.Host, "port": node.Port, "users": []map[string]interface{}{{"id": node.UUID, "encryption": "none"}}}}}, StreamSettings: map[string]interface{}{"network": "ws", "security": "tls", "tlsSettings": map[string]interface{}{"serverName": node.SNI, "allowInsecure": false}, "wsSettings": map[string]interface{}{"path": node.WSPath, "headers": map[string]string{"Host": node.WSHost}}}}
	case "trojan:tcp":
		ob = outbound{Tag: "proxy", Protocol: "trojan", Settings: map[string]interface{}{"servers": []map[string]interface{}{{"address": node.Host, "port": node.Port, "password": node.Password}}}, StreamSettings: map[string]interface{}{"network": "tcp", "security": "tls", "tlsSettings": map[string]interface{}{"serverName": node.SNI, "allowInsecure": false}}}
	case "trojan:ws":
		ob = outbound{Tag: "proxy", Protocol: "trojan", Settings: map[string]interface{}{"servers": []map[string]interface{}{{"address": node.Host, "port": node.Port, "password": node.Password}}}, StreamSettings: map[string]interface{}{"network": "ws", "security": "tls", "tlsSettings": map[string]interface{}{"serverName": node.SNI, "allowInsecure": false}, "wsSettings": map[string]interface{}{"path": node.WSPath, "headers": map[string]string{"Host": node.WSHost}}}}
	default:
		return errors.New("unsupported_transport")
	}

	cfg["outbounds"] = []outbound{ob}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func randomPort() (int, error) {
	for i := 0; i < 10; i++ {
		port := 30000 + secureRandInt(10000)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = ln.Close()
			return port, nil
		}
	}
	return 0, errors.New("unable to allocate port")
}

func secureRandInt(max int) int {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return int(time.Now().UnixNano() % int64(max))
	}
	return int(binary.BigEndian.Uint64(b[:]) % uint64(max))
}

func waitForProxy(cmd *exec.Cmd, port int, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	deadline := time.Now().Add(timeout)
	address := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return 0, errors.New("xray_exited_early")
		}
		conn, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return time.Since(start), nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0, errors.New("proxy_not_ready")
}

func tailLog(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 6 {
		lines = lines[len(lines)-6:]
	}
	return strings.Join(lines, " | "), nil
}

func probeURL(rawURL string, proxyPort int, timeout time.Duration) (int, error) {
	if _, err := url.Parse(rawURL); err != nil {
		return 0, err
	}
	baseDialer := &net.Dialer{Timeout: timeout}
	socksDialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), nil, baseDialer)
	if err != nil {
		return 0, err
	}
	contextDialer, ok := socksDialer.(proxy.ContextDialer)
	if !ok {
		return 0, errors.New("socks_context_dialer_unavailable")
	}
	transport := &http.Transport{DialContext: contextDialer.DialContext, TLSHandshakeTimeout: timeout, DisableKeepAlives: true}
	client := &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func logf(verbose bool, format string, args ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[INFO] "+format+"\n", args...)
	}
}
