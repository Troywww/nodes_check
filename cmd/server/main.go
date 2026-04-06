package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nodes_check/internal/app"
	"nodes_check/internal/config"
	webui "nodes_check/internal/web"
)

func main() {
	configPath := flag.String("config", "./configs/config.example.yaml", "config path")
	once := flag.Bool("once", false, "run pipeline once and exit")
	flag.Parse()

	runner := app.NewRunner(*configPath)

	if *once {
		if err := runner.RunAsync(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for {
			state := runner.State()
			if !state.Running {
				if state.LastError != "" {
					fmt.Fprintln(os.Stderr, state.LastError)
					os.Exit(1)
				}
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runner.StartScheduledLoop(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           webui.New(runner, *configPath).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("nodes-check web listening on %s", cfg.Server.Listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
