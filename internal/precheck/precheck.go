package precheck

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"nodes_check/internal/classifier"
)

type Result struct {
	classifier.Node
	TCPReachable bool
	TCPDelayMS   int64
	SkipReason   string
	Error        string
}

func Deduplicate(nodes []classifier.Node) []classifier.Node {
	seen := make(map[string]struct{}, len(nodes))
	deduped := make([]classifier.Node, 0, len(nodes))
	for _, node := range nodes {
		key := fmt.Sprintf("%s:%d", node.Host, node.Port)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, node)
	}
	return deduped
}

func Run(ctx context.Context, nodes []classifier.Node, timeout time.Duration, concurrency int) []Result {
	if concurrency < 1 {
		concurrency = 1
	}
	results := make([]Result, 0, len(nodes))
	resultCh := make(chan Result, len(nodes))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, node := range nodes {
		node := node
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			resultCh <- checkOne(ctx, node, timeout)
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Category != results[j].Category {
			return results[i].Category < results[j].Category
		}
		return results[i].TCPDelayMS < results[j].TCPDelayMS
	})
	return results
}

func checkOne(ctx context.Context, node classifier.Node, timeout time.Duration) Result {
	result := Result{Node: node}
	if classifier.IsDNSCategory(node) && node.Port != 443 {
		result.SkipReason = "dns_category_requires_443"
		return result
	}

	dialer := net.Dialer{Timeout: timeout}
	target := fmt.Sprintf("%s:%d", node.Host, node.Port)
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", target)
	result.TCPDelayMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	_ = conn.Close()
	result.TCPReachable = true
	return result
}
