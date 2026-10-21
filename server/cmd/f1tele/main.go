// Command f1tele serves the telemetry dashboard, recorder and race engineer.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"f1tele/server/internal/auth"
	"f1tele/server/internal/config"
	"f1tele/server/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	if os.Args[1] == "auth" {
		authCommand(os.Args[2:])
		return
	}
	if os.Args[1] != "serve" {
		usage()
	}
	serve(os.Args[2:])
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: f1tele serve [--config path] [--data-dir path] [--http addr] [--udp addr]")
	fmt.Fprintln(os.Stderr, "       f1tele auth ensure --json [--data-dir path]")
	fmt.Fprintln(os.Stderr, "       f1tele auth reset [--stdin] [--data-dir path]")
	os.Exit(2)
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "config.json path")
	dataDir := fs.String("data-dir", "", "data directory")
	httpAddr := fs.String("http", "", "HTTP listen address (host:port)")
	udpAddr := fs.String("udp", "", "UDP listen address (host:port)")
	_ = fs.Parse(args)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	store, err := config.Load(config.Overrides{HTTPAddress: *httpAddr, UDPAddress: *udpAddr, DataDir: *dataDir, ConfigPath: *configPath})
	if err != nil {
		logger.Error("configuration", "err", err)
		os.Exit(1)
	}
	authStore := auth.New(store.DataDir())
	first, err := authStore.Ensure()
	if err != nil {
		logger.Error("authentication", "err", err)
		os.Exit(1)
	}
	if first.Created {
		fmt.Fprintf(os.Stderr, "F1Telemetry web password (shown once): %s\n", first.Password)
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

func authCommand(args []string) {
	if len(args) < 1 {
		usage()
	}
	command := args[0]
	fs := flag.NewFlagSet("auth "+command, flag.ExitOnError)
	dataDir := fs.String("data-dir", "", "data directory")
	stdin := fs.Bool("stdin", false, "read the new password from standard input without confirmation")
	jsonOut := fs.Bool("json", false, "print a machine-readable result")
	_ = fs.Parse(args[1:])
	dir := config.ResolveDataDir(strings.TrimSpace(*dataDir))
	store := auth.New(dir)
	switch command {
	case "ensure":
		res, err := store.Ensure()
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth ensure:", err)
			os.Exit(1)
		}
		if *jsonOut {
			raw, _ := json.Marshal(res)
			fmt.Println(string(raw))
			return
		}
		if res.Created {
			fmt.Printf("Generated web password (shown once): %s\n", res.Password)
		} else {
			fmt.Printf("Web password already %s.\n", res.Source)
		}
	case "reset":
		password, err := readNewPassword(*stdin, os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth reset:", err)
			os.Exit(1)
		}
		if err := store.Reset(password); err != nil {
			fmt.Fprintln(os.Stderr, "auth reset:", err)
			os.Exit(1)
		}
		fmt.Println("Web password reset. Existing sessions were invalidated.")
	default:
		usage()
	}
}

func readNewPassword(fromStdin bool, input io.Reader) (string, error) {
	reader := bufio.NewReader(input)
	if fromStdin {
		line, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return "", fmt.Errorf("read password: %v", err)
		}
		password := strings.TrimRight(line, "\r\n")
		return password, auth.ValidatePassword(password)
	}
	fmt.Fprint(os.Stderr, "New web password: ")
	first, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(first) == "" {
		return "", err
	}
	first = strings.TrimRight(first, "\r\n")
	fmt.Fprint(os.Stderr, "Confirm new password: ")
	second, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(second) == "" {
		return "", err
	}
	if first != strings.TrimRight(second, "\r\n") {
		return "", fmt.Errorf("passwords do not match")
	}
	return first, auth.ValidatePassword(first)
}
