package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	iprobe "nodes_check/internal/probe"
)

type cliConfig struct {
	Node           string
	File           string
	OutputFile     string
	Concurrency    int
	ProbeURL       string
	Timeout        time.Duration
	StartupTimeout time.Duration
	Repeat         int
	XrayBin        string
	JSONOutput     bool
	Verbose        bool
}

func main() {
	cfg := parseFlags()
	if err := validateFlags(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	probeCfg := iprobe.Config{
		ProbeURL:       cfg.ProbeURL,
		Timeout:        cfg.Timeout,
		StartupTimeout: cfg.StartupTimeout,
		Repeat:         cfg.Repeat,
		Concurrency:    cfg.Concurrency,
		XrayBin:        cfg.XrayBin,
		Verbose:        cfg.Verbose,
	}
	if err := iprobe.ValidateConfig(probeCfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if cfg.Node != "" {
		aggregate := iprobe.RunOne(probeCfg, cfg.Node)
		if cfg.Repeat == 1 {
			renderSingle(cfg, aggregate.Results[0])
			if aggregate.Summary.SuccessCount == 0 {
				os.Exit(1)
			}
			return
		}
		renderValue(cfg, aggregate)
		if aggregate.Summary.SuccessCount == 0 {
			os.Exit(1)
		}
		return
	}

	output, err := iprobe.RunBatch(probeCfg, cfg.File)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	data, err := json.Marshal(output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.OutputFile != "" {
		if err := os.WriteFile(cfg.OutputFile, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if cfg.JSONOutput || cfg.OutputFile == "" {
		fmt.Println(string(data))
	}
	if !hasBatchSuccess(output) {
		os.Exit(1)
	}
}

func parseFlags() cliConfig {
	cfg := cliConfig{}
	flag.StringVar(&cfg.Node, "node", "", "single node URL")
	flag.StringVar(&cfg.File, "file", "", "file containing one node URL per line")
	flag.StringVar(&cfg.OutputFile, "output", "", "write batch json output to file")
	flag.IntVar(&cfg.Concurrency, "concurrency", 3, "concurrency for file mode")
	flag.StringVar(&cfg.ProbeURL, "probe-url", "https://www.gstatic.com/generate_204", "probe URL")
	timeout := flag.Int("timeout", 8, "request timeout in seconds")
	startupTimeout := flag.Int("startup-timeout", 4, "startup timeout in seconds")
	flag.IntVar(&cfg.Repeat, "repeat", 1, "repeat count")
	flag.StringVar(&cfg.XrayBin, "xray-bin", "./bin/xray-linux-64/xray", "xray binary path")
	flag.BoolVar(&cfg.JSONOutput, "json", false, "output json")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "verbose logs")
	flag.Parse()
	cfg.Timeout = time.Duration(*timeout) * time.Second
	cfg.StartupTimeout = time.Duration(*startupTimeout) * time.Second
	return cfg
}

func validateFlags(cfg cliConfig) error {
	if cfg.Node != "" && cfg.File != "" {
		return errors.New("use either --node or --file")
	}
	if cfg.Node == "" && cfg.File == "" {
		return errors.New("missing --node or --file")
	}
	if cfg.Repeat < 1 {
		return errors.New("--repeat must be >= 1")
	}
	if cfg.Concurrency < 1 {
		return errors.New("--concurrency must be >= 1")
	}
	return nil
}

func renderSingle(cfg cliConfig, result iprobe.SingleResult) {
	if cfg.JSONOutput {
		renderValue(cfg, result)
		return
	}
	fmt.Printf("Node: %s\n", result.NodeName)
	fmt.Printf("Protocol: %s\n", result.Protocol)
	fmt.Printf("Endpoint: %s:%d\n", result.Host, result.Port)
	fmt.Printf("Probe URL: %s\n", result.ProbeURL)
	fmt.Printf("Success: %t\n", result.Success)
	fmt.Printf("Proxy Ready: %t\n", result.ProxyReady)
	fmt.Printf("Status Code: %d\n", result.StatusCode)
	fmt.Printf("Real Delay: %d ms\n", result.RealDelayMS)
	fmt.Printf("Fail Reason: %s\n\n", result.FailReason)
}

func renderValue(cfg cliConfig, value interface{}) {
	data, _ := json.Marshal(value)
	fmt.Println(string(data))
}

func hasBatchSuccess(output iprobe.BatchOutput) bool {
	for _, item := range output.Items {
		if item.OK {
			return true
		}
	}
	return false
}
