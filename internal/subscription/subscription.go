package subscription

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Source struct {
	URL string `json:"url"`
}

type Fetched struct {
	URL       string `json:"url"`
	Content   string `json:"content"`
	Decoded   bool   `json:"decoded"`
	LineCount int    `json:"line_count"`
}

func LoadSourcesFromFile(path string) ([]Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	sources := make([]Source, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sources = append(sources, Source{URL: line})
	}
	if len(sources) == 0 {
		return nil, errors.New("no subscription sources found")
	}
	return sources, nil
}

func Fetch(ctx context.Context, sourceURL string, timeout time.Duration) (Fetched, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return Fetched{}, err
	}
	req.Header.Set("User-Agent", "nodes-check/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return Fetched{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Fetched{}, err
	}

	content := strings.TrimSpace(string(body))
	decoded := false
	if !strings.Contains(content, "://") {
		if buf, err := base64.StdEncoding.DecodeString(content); err == nil {
			content = strings.TrimSpace(string(buf))
			decoded = true
		}
	}

	lines := 0
	if content != "" {
		lines = len(strings.Split(content, "\n"))
	}

	return Fetched{
		URL:       sourceURL,
		Content:   content,
		Decoded:   decoded,
		LineCount: lines,
	}, nil
}
