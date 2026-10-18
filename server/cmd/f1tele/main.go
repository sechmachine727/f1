// Command f1tele serves the telemetry dashboard, recorder and race engineer.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"f1tele/server/internal/config"
	"f1tele/server/internal/server"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: f1tele serve [--config path] [--data-dir path] [--http addr] [--udp addr]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "config.json path")
	dataDir := fs.String("data-dir", "", "data directory")
	httpAddr := fs.String("http", "", "HTTP listen address (host:port)")
	udpAddr := fs.String("udp", "", "UDP listen address (host:port)")
	_ = fs.Parse(os.Args[2:])

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	store, err := config.Load(config.Overrides{HTTPAddress: *httpAddr, UDPAddress: *udpAddr, DataDir: *dataDir, ConfigPath: *configPath})
	if err != nil {
		logger.Error("configuration", "err", err)
		os.Exit(1)
	}
	srv, err := server.New(store, logger)
	if err != nil {
		logger.Error("startup", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Run(ctx, logger); err != nil {
		logger.Error("run", "err", err)
		srv.Shutdown(10e9)
		os.Exit(1)
	}
	srv.Shutdown(10e9)
}
