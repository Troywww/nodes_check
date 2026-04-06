package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"nodes_check/internal/config"
	"nodes_check/internal/selector"
)

type Result struct {
	KV       KVResult    `json:"kv"`
	DNS      []DNSResult `json:"dns"`
	HasError bool        `json:"has_error"`
}

type KVResult struct {
	Enabled bool   `json:"enabled"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type DNSResult struct {
	Category string `json:"category"`
	Domain   string `json:"domain"`
	IP       string `json:"ip"`
	Enabled  bool   `json:"enabled"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
}

func Publish(ctx context.Context, cfg config.Config, kvContent string, dnsItems []selector.Selected) Result {
	result := Result{}
	if cfg.Publish.KV.Enabled {
		result.KV = publishKV(ctx, cfg, kvContent)
		if !result.KV.OK {
			result.HasError = true
		}
	} else {
		result.KV = KVResult{Enabled: false, OK: true, Message: "kv publish disabled"}
	}

	dnsResults := publishDNS(ctx, cfg, dnsItems)
	result.DNS = dnsResults
	for _, item := range dnsResults {
		if item.Enabled && !item.OK {
			result.HasError = true
		}
	}
	return result
}

func publishKV(ctx context.Context, cfg config.Config, content string) KVResult {
	if strings.TrimSpace(content) == "" {
		return KVResult{Enabled: true, OK: true, Message: "kv content is empty"}
	}
	if cfg.Cloudflare.Worker.URL == "" {
		return KVResult{Enabled: true, OK: false, Message: "worker url is required"}
	}
	if cfg.Cloudflare.Worker.Key == "" {
		return KVResult{Enabled: true, OK: false, Message: "worker key is required"}
	}

	base := strings.TrimSpace(cfg.Cloudflare.Worker.URL)
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}
	base = strings.TrimRight(base, "/")

	encodedKey := url.PathEscape(cfg.Cloudflare.Worker.Key)
	textValue := url.QueryEscape(content)
	endpoint := fmt.Sprintf("%s/%s?token=%s&text=%s", base, encodedKey, url.QueryEscape(cfg.Cloudflare.Worker.Token), textValue)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return KVResult{Enabled: true, OK: false, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return KVResult{Enabled: true, OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	body := strings.TrimSpace(string(data))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return KVResult{Enabled: true, OK: false, Message: fmt.Sprintf("status=%d body=%s", resp.StatusCode, body)}
	}
	return KVResult{Enabled: true, OK: true, Message: fmt.Sprintf("status=%d body=%s", resp.StatusCode, body)}
}

func publishDNS(ctx context.Context, cfg config.Config, items []selector.Selected) []DNSResult {
	if !cfg.Publish.DNS.Enabled {
		results := make([]DNSResult, 0, len(cfg.Publish.DNS.Categories))
		for category, count := range cfg.Publish.DNS.Categories {
			if count < 0 {
				continue
			}
			results = append(results, DNSResult{Category: category, Domain: cfg.Cloudflare.DNS.Domains[category], Enabled: false, OK: true, Message: "dns publish disabled"})
		}
		sort.Slice(results, func(i, j int) bool { return results[i].Category < results[j].Category })
		return results
	}

	grouped := make(map[string][]selector.Selected)
	for _, item := range items {
		grouped[item.Precheck.Category] = append(grouped[item.Precheck.Category], item)
	}
	for category := range grouped {
		sort.Slice(grouped[category], func(i, j int) bool {
			return grouped[category][i].Best.RealDelayMS < grouped[category][j].Best.RealDelayMS
		})
	}

	categories := make([]string, 0, len(cfg.Publish.DNS.Categories))
	for category := range cfg.Publish.DNS.Categories {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	results := make([]DNSResult, 0, len(categories))
	for _, category := range categories {
		desired := cfg.Publish.DNS.Categories[category]
		domain := strings.TrimSpace(cfg.Cloudflare.DNS.Domains[category])
		if domain == "" {
			results = append(results, DNSResult{Category: category, Enabled: true, OK: false, Message: "dns domain is required"})
			continue
		}
		if desired <= 0 {
			results = append(results, DNSResult{Category: category, Domain: domain, Enabled: true, OK: true, Message: "dns count is 0"})
			continue
		}
		itemsForCategory := grouped[category]
		if len(itemsForCategory) == 0 {
			results = append(results, DNSResult{Category: category, Domain: domain, Enabled: true, OK: true, Message: "no selected node for category"})
			continue
		}
		if len(itemsForCategory) > desired {
			itemsForCategory = itemsForCategory[:desired]
		}
		ips := make([]string, 0, len(itemsForCategory))
		for _, item := range itemsForCategory {
			if net.ParseIP(item.Precheck.Host) == nil {
				continue
			}
			ips = append(ips, item.Precheck.Host)
		}
		if len(ips) == 0 {
			results = append(results, DNSResult{Category: category, Domain: domain, Enabled: true, OK: false, Message: "no valid IPv4 selected for category"})
			continue
		}
		dnsResult := syncDNSRecords(ctx, cfg, domain, ips)
		dnsResult.Category = category
		dnsResult.Domain = domain
		dnsResult.IP = strings.Join(ips, ",")
		results = append(results, dnsResult)
	}
	return results
}

func syncDNSRecords(ctx context.Context, cfg config.Config, domain string, ips []string) DNSResult {
	if cfg.Cloudflare.DNS.APIToken == "" {
		return DNSResult{Enabled: true, OK: false, Message: "dns api token is required"}
	}
	if cfg.Cloudflare.DNS.ZoneID == "" {
		return DNSResult{Enabled: true, OK: false, Message: "dns zone id is required"}
	}

	client := &http.Client{Timeout: 20 * time.Second}
	listURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=A&name=%s", cfg.Cloudflare.DNS.ZoneID, url.QueryEscape(domain))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return DNSResult{Enabled: true, OK: false, Message: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Cloudflare.DNS.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return DNSResult{Enabled: true, OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return DNSResult{Enabled: true, OK: false, Message: fmt.Sprintf("list status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}

	var listed struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Result []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		return DNSResult{Enabled: true, OK: false, Message: err.Error()}
	}
	if !listed.Success && len(listed.Errors) > 0 {
		return DNSResult{Enabled: true, OK: false, Message: listed.Errors[0].Message}
	}

	for i, ip := range ips {
		payload := fmt.Sprintf(`{"type":"A","name":"%s","content":"%s","ttl":1,"proxied":false}`, domain, ip)
		method := http.MethodPost
		targetURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", cfg.Cloudflare.DNS.ZoneID)
		if i < len(listed.Result) && listed.Result[i].ID != "" {
			method = http.MethodPut
			targetURL = fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", cfg.Cloudflare.DNS.ZoneID, listed.Result[i].ID)
		}
		writeReq, err := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(payload))
		if err != nil {
			return DNSResult{Enabled: true, OK: false, Message: err.Error()}
		}
		writeReq.Header.Set("Authorization", "Bearer "+cfg.Cloudflare.DNS.APIToken)
		writeReq.Header.Set("Content-Type", "application/json")
		writeResp, err := client.Do(writeReq)
		if err != nil {
			return DNSResult{Enabled: true, OK: false, Message: err.Error()}
		}
		body, _ := io.ReadAll(io.LimitReader(writeResp.Body, 2048))
		writeResp.Body.Close()
		if writeResp.StatusCode < 200 || writeResp.StatusCode >= 300 {
			return DNSResult{Enabled: true, OK: false, Message: fmt.Sprintf("write status=%d body=%s", writeResp.StatusCode, strings.TrimSpace(string(body)))}
		}
	}

	for i := len(ips); i < len(listed.Result); i++ {
		if listed.Result[i].ID == "" {
			continue
		}
		deleteURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", cfg.Cloudflare.DNS.ZoneID, listed.Result[i].ID)
		deleteReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
		if err != nil {
			return DNSResult{Enabled: true, OK: false, Message: err.Error()}
		}
		deleteReq.Header.Set("Authorization", "Bearer "+cfg.Cloudflare.DNS.APIToken)
		deleteResp, err := client.Do(deleteReq)
		if err != nil {
			return DNSResult{Enabled: true, OK: false, Message: err.Error()}
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(deleteResp.Body, 2048))
		deleteResp.Body.Close()
		if deleteResp.StatusCode < 200 || deleteResp.StatusCode >= 300 {
			return DNSResult{Enabled: true, OK: false, Message: fmt.Sprintf("delete status=%d", deleteResp.StatusCode)}
		}
	}

	return DNSResult{Enabled: true, OK: true, Message: fmt.Sprintf("records=%d", len(ips))}
}
