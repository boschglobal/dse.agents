// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boschglobal/dse.agents/mcp/api"
	"github.com/boschglobal/dse.agents/mcp/pkg/runtime"
	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mark3labs/mcp-go/server"
)

func normalizeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "-v":
			out = append(out, "-v=1")
		case "-vv":
			out = append(out, "-v=2")
		case "-vvv":
			out = append(out, "-v=3")
		default:
			out = append(out, arg)
		}
	}
	return out
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	verbosity := fs.Int("v", 0, "Verbosity level")
	httpMode := fs.Bool("http", false, "Run in HTTP-only mode.")
	mcpMode := fs.String("mcp", "http", "MCP Server binds to 'http' (default) or 'stdio'.")
	if err := fs.Parse(normalizeArgs(os.Args[1:])); err != nil {
		slog.Error("failed to parse flags", "err", err, "args", os.Args[1:])
		os.Exit(2)
	}

	var level slog.Level
	switch *verbosity {
	case 1:
		level = slog.LevelDebug
	default:
		level = slog.LevelDebug
	}
	slog.SetLogLoggerLevel(level)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Agent Services (to be provided by MCP/Huma).
	idleTimeout := 30 * time.Minute
	scanInterval := 5 * time.Minute
	sm := session.NewManager(ctx, idleTimeout, scanInterval)

	runtime, _ := runtime.NewRuntime("docker")
	cts, _ := service.NewContainerTaskService(ctx, runtime)
	vts := service.NewVolumeTaskService(runtime)

	sm.SetVolumeTeardown(func(ctx context.Context, sessionID string, vol *volume.Volume) error {
		return runtime.DeleteVolume(
			ctx,
			sessionID,
			vol,
			service.Progress(func(taskID string, log service.Log) {
				slog.Debug("Session: volume cleanup progress", "sessionID", sessionID, "vol.Name", vol.Name)
			}))
	})

	// Huma
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	h := humachi.New(r, huma.DefaultConfig("Automation Engine", "0.1.0")) // TODO version
	api.RegisterHumaRoutes(h, r, cts)
	srv := &http.Server{
		Addr:    ":8888",
		Handler: r,
	}
	go func() {
		<-ctx.Done()
		slog.Info("Shutting down Huma server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Huma server shutdown failed", "err", err)
		}
	}()
	go func() {
		slog.Info("Huma listening at http://localhost:8888/")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP Server crash", "err", err)
			os.Exit(1)
		}
	}()
	if *httpMode {
		// Only run this HTTP interface.
		<-ctx.Done()
		return
	}

	// MCP Server.
	// TODO: more handlers.
	// mcpServer := server.NewMCPServer("Multi-Volume-Server", "1.0.0")
	// handlers := api.NewMCPHandlers(sessionMgr, runner, mover)
	// handlers.RegisterVolumeTools(mcpServer)
	mcpServer, _ := api.CreateMcpServer(sm, cts, vts)
	if *mcpMode == "stdio" {
		slog.Info("MCP Server running on stdio")
		if err := server.ServeStdio(mcpServer); err != nil {
			slog.Error("MCP Server crash", "err", err)
			os.Exit(1)
		}
	} else {
		slog.Info("MCP Server running on HTTP", "port", "8080", "path", "/mcp")
		httpHandler := server.NewStreamableHTTPServer(mcpServer)
		mcpHTTPServer := &http.Server{
			Addr:    ":8080",
			Handler: httpHandler,
		}
		go func() {
			<-ctx.Done()
			slog.Info("Shutting down MCP HTTP server")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := mcpHTTPServer.Shutdown(shutdownCtx); err != nil {
				slog.Error("MCP HTTP server shutdown failed", "err", err)
			}
		}()
		if err := mcpHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("MCP Server network crash", "err", err)
			os.Exit(1)
		}

		<-ctx.Done()
	}
}
