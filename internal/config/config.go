package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Listen   string `yaml:"listen"`
		Timezone string `yaml:"timezone"`
	} `yaml:"server"`
	Web struct {
		AuthToken string `yaml:"auth_token"`
	} `yaml:"web"`
	Schedule struct {
		Enabled         bool `yaml:"enabled"`
		IntervalMinutes int  `yaml:"interval_minutes"`
	} `yaml:"schedule"`
	Job struct {
		Concurrency           int `yaml:"concurrency"`
		Repeat                int `yaml:"repeat"`
		MaxProbeCandidates    int `yaml:"max_probe_candidates"`
		TCPPrecheckTimeoutSec int `yaml:"tcp_precheck_timeout_seconds"`
		ProbeTimeoutSec       int `yaml:"probe_timeout_seconds"`
		StartupTimeoutSec     int `yaml:"startup_timeout_seconds"`
	} `yaml:"job"`
	Subscription struct {
		File string `yaml:"file"`
	} `yaml:"subscription"`
	Probe struct {
		XrayBin  string `yaml:"xray_bin"`
		ProbeURL string `yaml:"probe_url"`
		Template struct {
			Protocol  string `yaml:"protocol"`
			UUID      string `yaml:"uuid"`
			Password  string `yaml:"password"`
			Security  string `yaml:"security"`
			Transport string `yaml:"transport"`
			SNI       string `yaml:"sni"`
			WSHost    string `yaml:"ws_host"`
			WSPath    string `yaml:"ws_path"`
		} `yaml:"template"`
		Phase1MaxDelayMS int64 `yaml:"phase1_max_delay_ms"`
		Phase2MaxDelayMS int64 `yaml:"phase2_max_delay_ms"`
	} `yaml:"probe"`
	Output struct {
		Dir        string `yaml:"dir"`
		NamePrefix string `yaml:"name_prefix"`
	} `yaml:"output"`
	Cache struct {
		HistoryFile string `yaml:"history_file"`
	} `yaml:"cache"`
	Publish struct {
		KV struct {
			Enabled    bool           `yaml:"enabled"`
			Categories map[string]int `yaml:"categories"`
		} `yaml:"kv"`
		DNS struct {
			Enabled    bool           `yaml:"enabled"`
			Categories map[string]int `yaml:"categories"`
		} `yaml:"dns"`
	} `yaml:"publish"`
	Cloudflare struct {
		Worker struct {
			URL   string `yaml:"url"`
			Token string `yaml:"token"`
			Key   string `yaml:"key"`
		} `yaml:"worker"`
		DNS struct {
			APIToken string            `yaml:"api_token"`
			ZoneID   string            `yaml:"zone_id"`
			Domains  map[string]string `yaml:",inline"`
		} `yaml:"dns"`
	} `yaml:"cloudflare"`
}

func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.normalize(path)
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) normalize(path string) {
	baseDir := filepath.Dir(path)
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	if c.Server.Timezone == "" {
		c.Server.Timezone = "Asia/Shanghai"
	}
	if c.Job.Concurrency <= 0 {
		c.Job.Concurrency = 3
	}
	if c.Job.Repeat <= 0 {
		c.Job.Repeat = 1
	}
	if c.Job.MaxProbeCandidates <= 0 {
		c.Job.MaxProbeCandidates = 60
	}
	if c.Job.TCPPrecheckTimeoutSec <= 0 {
		c.Job.TCPPrecheckTimeoutSec = 3
	}
	if c.Job.ProbeTimeoutSec <= 0 {
		c.Job.ProbeTimeoutSec = 8
	}
	if c.Job.StartupTimeoutSec <= 0 {
		c.Job.StartupTimeoutSec = 4
	}
	if c.Schedule.IntervalMinutes <= 0 {
		c.Schedule.IntervalMinutes = 60
	}
	if c.Subscription.File != "" && !filepath.IsAbs(c.Subscription.File) {
		c.Subscription.File = filepath.Clean(filepath.Join(baseDir, c.Subscription.File))
	}
	if c.Probe.XrayBin != "" && !filepath.IsAbs(c.Probe.XrayBin) {
		c.Probe.XrayBin = filepath.Clean(filepath.Join(baseDir, c.Probe.XrayBin))
	}
	if c.Probe.Template.Protocol == "" {
		c.Probe.Template.Protocol = "vless"
	}
	if c.Probe.Template.Security == "" {
		c.Probe.Template.Security = "tls"
	}
	if c.Probe.Template.Transport == "" {
		c.Probe.Template.Transport = "ws"
	}
	if c.Probe.Template.WSPath == "" {
		c.Probe.Template.WSPath = "/"
	}
	if c.Probe.Phase1MaxDelayMS <= 0 {
		c.Probe.Phase1MaxDelayMS = 1500
	}
	if c.Probe.Phase2MaxDelayMS <= 0 {
		c.Probe.Phase2MaxDelayMS = 2500
	}
	if c.Output.Dir == "" {
		c.Output.Dir = filepath.Clean(filepath.Join(baseDir, "..", "runtime", "outputs"))
	} else if !filepath.IsAbs(c.Output.Dir) {
		c.Output.Dir = filepath.Clean(filepath.Join(baseDir, c.Output.Dir))
	}
	if c.Cache.HistoryFile == "" {
		c.Cache.HistoryFile = filepath.Clean(filepath.Join(baseDir, "..", "runtime", "cache", "history_valid_nodes.txt"))
	} else if !filepath.IsAbs(c.Cache.HistoryFile) {
		c.Cache.HistoryFile = filepath.Clean(filepath.Join(baseDir, c.Cache.HistoryFile))
	}
	if c.Publish.KV.Categories == nil {
		c.Publish.KV.Categories = map[string]int{}
	}
	if c.Publish.DNS.Categories == nil {
		c.Publish.DNS.Categories = map[string]int{}
	}
	if c.Cloudflare.DNS.Domains == nil {
		c.Cloudflare.DNS.Domains = map[string]string{}
	}
	delete(c.Cloudflare.DNS.Domains, "api_token")
	delete(c.Cloudflare.DNS.Domains, "zone_id")
}

func (c Config) Validate() error {
	if c.Subscription.File == "" {
		return errors.New("subscription.file is required")
	}
	if c.Probe.XrayBin == "" {
		return errors.New("probe.xray_bin is required")
	}
	if c.Probe.ProbeURL == "" {
		return errors.New("probe.probe_url is required")
	}
	if c.Output.Dir == "" {
		return errors.New("output.dir is required")
	}
	if c.Cache.HistoryFile == "" {
		return errors.New("cache.history_file is required")
	}
	if c.Web.AuthToken == "" {
		return errors.New("web.auth_token is required")
	}
	if c.Probe.Template.Protocol == "vless" && c.Probe.Template.UUID == "" {
		return errors.New("probe.template.uuid is required for vless")
	}
	if c.Probe.Template.Protocol == "trojan" && c.Probe.Template.Password == "" {
		return errors.New("probe.template.password is required for trojan")
	}
	return nil
}

func (c Config) ProbeTimeout() time.Duration {
	return time.Duration(c.Job.ProbeTimeoutSec) * time.Second
}

func (c Config) StartupTimeout() time.Duration {
	return time.Duration(c.Job.StartupTimeoutSec) * time.Second
}

func (c Config) TCPPrecheckTimeout() time.Duration {
	return time.Duration(c.Job.TCPPrecheckTimeoutSec) * time.Second
}
