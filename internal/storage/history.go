package storage

import (
	"os"
	"path/filepath"
	"strings"

	"nodes_check/internal/parser"
	"nodes_check/internal/selector"
)

func LoadHistory(path string) ([]parser.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	nodes := make([]parser.Node, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := parser.ParseHistoryLine(line)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func SaveHistory(path string, items []selector.Candidate) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, parser.FormatHostPortName(item.Precheck.Node.Node))
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
