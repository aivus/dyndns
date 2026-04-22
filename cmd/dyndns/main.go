package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/aivus/dyndns/internal/config"
	"github.com/aivus/dyndns/internal/handler"
	"github.com/aivus/dyndns/internal/updater"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

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

	slog.Info("starting server", "addr", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
