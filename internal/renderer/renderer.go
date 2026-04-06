package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nodes_check/internal/classifier"
	"nodes_check/internal/selector"
)

type OutputFiles struct {
	Dir     string
	FinalIP string
}

func WriteOutputs(dir string, items []selector.Selected, kvDomains map[string]string) (OutputFiles, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return OutputFiles{}, err
	}

	files := OutputFiles{
		Dir:     dir,
		FinalIP: filepath.Join(dir, "final_ips.txt"),
	}

	if err := writeFinalIPFile(files.FinalIP, items, kvDomains); err != nil {
		return OutputFiles{}, err
	}
	return files, nil
}

func WriteLines(path string, items []selector.Selected, kvDomains map[string]string) error {
	return writeFinalIPFile(path, items, kvDomains)
}

func writeFinalIPFile(path string, items []selector.Selected, kvDomains map[string]string) error {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		name := item.FinalName
		if name == "" {
			name = item.Precheck.Name
		}
		host := item.Precheck.Host
		switch item.Precheck.Category {
		case classifier.CategoryMobile, classifier.CategoryUnicom, classifier.CategoryTelecom, classifier.CategoryOfficial:
			if domain := strings.TrimSpace(kvDomains[item.Precheck.Category]); domain != "" {
				host = domain
			}
		}
		lines = append(lines, fmt.Sprintf("%s:%d#%s", host, item.Precheck.Port, name))
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
