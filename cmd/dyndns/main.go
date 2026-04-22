package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aivus/dyndns/internal/config"
	"github.com/aivus/dyndns/internal/handler"
	"github.com/aivus/dyndns/internal/updater"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	cfClient := updater.NewCloudflareClient(cfg.Cloudflare.APIToken)
	u := updater.New(cfClient, cfg.Records)
	h := handler.New(cfg.UpdateToken, u)

	mux := http.NewServeMux()
	mux.Handle("GET /update", h)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("starting server", "addr", *addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
