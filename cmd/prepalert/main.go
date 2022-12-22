package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/fujiwara/logutils"
	"github.com/handlename/ssmwrap"
	"github.com/mashiike/prepalert"
)

func main() {
	filter := &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"debug", "info", "notice", "warn", "error"},
		ModifierFuncs: []logutils.ModifierFunc{
			logutils.Color(color.FgHiBlack),
			nil,
			logutils.Color(color.FgHiBlue),
			logutils.Color(color.FgYellow),
			logutils.Color(color.FgRed, color.BgBlack),
		},
		MinLevel: "info",
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)
	ssmwrapPaths := os.Getenv("SSMWRAP_PATHS")
	paths := strings.Split(ssmwrapPaths, ",")
	if ssmwrapPaths != "" && len(paths) > 0 {
		err := ssmwrap.Export(ssmwrap.ExportOptions{
			Paths:   paths,
			Retries: 3,
		})
		if err != nil {
			log.Fatalf("[error] %v", err)
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
			log.Fatalf("[error] %v", err)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()
	if err := prepalert.RunCLI(ctx, os.Args[1:], func(logLevel string) { filter.SetMinLevel(logutils.LogLevel(logLevel)) }); err != nil {
		log.Fatalf("[error] %v", err)
	}
}
