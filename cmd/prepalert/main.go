package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/fujiwara/logutils"
	"github.com/handlename/ssmwrap"
	"github.com/mashiike/prepalert"
	"github.com/urfave/cli/v2"
)

var (
	Version = "current"
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
	i := &cli.App{
		Name:      "prepalert",
		Usage:     "A webhook server for prepare alert memo",
		UsageText: "prepalert -config <config file> [command options]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "config file path, can set multiple",
				EnvVars: []string{"CONFIG", "PREPALERT_CONFIG"},
				Value:   "config.yaml",
			},
			&cli.StringFlag{
				Name:        "mackerel-apikey",
				Aliases:     []string{"k"},
				Usage:       "for access mackerel API",
				DefaultText: "*********",
				EnvVars:     []string{"MACKEREL_APIKEY", "PREPALERT_MACKEREL_APIKEY"},
			},
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "output log-level",
				EnvVars: []string{"PREPALERT_LOG_LEVEL"},
				Value:   "info",
			},
			&cli.StringFlag{
				Name:    "address",
				Usage:   "run address",
				EnvVars: []string{"PREPALERT_ADDRESS"},
				Value:   ":8080",
			},
			&cli.StringFlag{
				Name:    "prefix",
				Usage:   "server prefix",
				EnvVars: []string{"PREPALERT_PREFIX"},
				Value:   "/",
			},
			&cli.StringFlag{
				Name:    "mode",
				Usage:   "run mode",
				EnvVars: []string{"PREPALERT_MODE"},
				Value:   "webhook",
			},
			&cli.IntFlag{
				Name:    "batch-size",
				Usage:   "local sqs batch size",
				EnvVars: []string{"PREPALERT_BATCH_SIZE"},
				Value:   1,
			},
		},
		EnableBashCompletion: true,
		Version:              Version,
		Before: func(ctx *cli.Context) error {
			filter.SetMinLevel(logutils.LogLevel(strings.ToLower(ctx.String("log-level"))))
			return nil
		},
		Action: func(ctx *cli.Context) error {
			cfg := prepalert.DefaultConfig()
			if err := cfg.Load(ctx.String("config")); err != nil {
				return err
			}
			app, err := prepalert.New(ctx.String("mackerel-apikey"), cfg)
			if err != nil {
				return err
			}
			return app.Run(ctx.Context, prepalert.RunOptions{
				Mode:      ctx.String("mode"),
				Address:   ctx.String("address"),
				Prefix:    ctx.String("prefix"),
				BatchSize: ctx.Int("batch-size"),
			})
		},
	}
	sort.Sort(cli.FlagsByName(i.Flags))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()
	if err := i.RunContext(ctx, os.Args); err != nil {
		log.Fatalf("[error] %v", err)
	}
}
