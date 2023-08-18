package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/handlename/ssmwrap"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/slogutils"
)

func main() {
	middleware := slogutils.NewMiddleware(
		slog.NewJSONHandler,
		slogutils.MiddlewareOptions{
			ModifierFuncs: map[slog.Level]slogutils.ModifierFunc{
				slog.LevelDebug: slogutils.Color(color.FgBlack),
				slog.LevelInfo:  nil,
				slog.LevelWarn:  slogutils.Color(color.FgYellow),
				slog.LevelError: slogutils.Color(color.FgRed, color.Bold),
			},
			RecordTransformerFuncs: []slogutils.RecordTransformerFunc{
				slogutils.DefaultAttrs(
					"version", prepalert.Version,
					"app", "prepalert",
				),
				slogutils.ConvertLegacyLevel(
					map[string]slog.Level{
						"debug":  slog.LevelDebug,
						"info":   slog.LevelInfo,
						"notice": slog.LevelInfo, // for backward compatibility
						"warn":   slog.LevelWarn,
						"error":  slog.LevelError,
					},
					false, // in-casesensitive
				),
			},
			Writer: os.Stderr,
			HandlerOptions: &slog.HandlerOptions{
				Level: slog.LevelWarn,
			},
		},
	)
	slog.SetDefault(slog.New(middleware))
	ssmwrapPaths := os.Getenv("SSMWRAP_PATHS")
	paths := strings.Split(ssmwrapPaths, ",")
	if ssmwrapPaths != "" && len(paths) > 0 {
		err := ssmwrap.Export(ssmwrap.ExportOptions{
			Paths:   paths,
			Retries: 3,
		})
		if err != nil {
			slog.Error(err.Error(), "issue", "ssmwrap_export")
			os.Exit(1)
		}
	}
	ssmwrapNames := os.Getenv("SSMWRAP_NAMES")
	names := strings.Split(ssmwrapNames, ",")
	if ssmwrapNames != "" && len(names) > 0 {
		err := ssmwrap.Export(ssmwrap.ExportOptions{
			Names:   names,
			Retries: 3,
		})
		if err != nil {
			slog.Error(err.Error(), "issue", "ssmwrap_export")
			os.Exit(1)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()
	setLogLevelFunc := func(logLevel string) {
		var l slog.Level
		switch strings.ToLower(logLevel) {
		case "debug":
			l = slog.LevelDebug
		case "info":
			l = slog.LevelInfo
		case "warn":
			l = slog.LevelWarn
		case "error":
			l = slog.LevelError
		default:
			l = slog.LevelInfo
		}
		middleware.SetMinLevel(l)
	}

	if err := prepalert.RunCLI(ctx, os.Args[1:], setLogLevelFunc); err != nil {
		slog.Error(err.Error(), "issue", "run_cli")
		os.Exit(1)
	}
}
