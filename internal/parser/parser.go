package parser

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"nodes_check/internal/subscription"
)

type Node struct {
	SourceURL string
	RawURL    string
	Name      string
	Host      string
	Port      int
}

func ParseFetched(fetched subscription.Fetched) ([]Node, error) {
	lines := strings.Split(fetched.Content, "\n")
	nodes := make([]Node, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := ParseSubscriptionLine(line, fetched.URL)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, errors.New("no supported nodes parsed")
	}
	return nodes, nil
}

func ParseSubscriptionLine(rawURL string, sourceURL string) (Node, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Node{}, err
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "vless" && scheme != "trojan" {
		return Node{}, errors.New("unsupported protocol")
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

	name := decodeName(u.Fragment)
	if name == "" {
		name = "unnamed"
	}

	return Node{
		SourceURL: sourceURL,
		RawURL:    rawURL,
		Name:      name,
		Host:      host,
		Port:      port,
	}, nil
}

func ParseHistoryLine(line string) (Node, error) {
	value := strings.TrimSpace(line)
	if value == "" {
		return Node{}, errors.New("empty line")
	}

	parts := strings.SplitN(value, "#", 2)
	endpoint := parts[0]
	name := "unnamed"
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		name = strings.TrimSpace(parts[1])
	}

	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		lastColon := strings.LastIndex(endpoint, ":")
		if lastColon <= 0 || lastColon == len(endpoint)-1 {
			return Node{}, fmt.Errorf("invalid endpoint: %s", endpoint)
		}
		host = endpoint[:lastColon]
		portStr = endpoint[lastColon+1:]
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Node{}, err
	}
	return Node{SourceURL: "history", Name: name, Host: host, Port: port}, nil
}

func FormatHostPortName(node Node) string {
	return fmt.Sprintf("%s:%d#%s", node.Host, node.Port, node.Name)
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
