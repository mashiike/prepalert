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
	a := &cli.App{
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
				Usage:   "run server prefix",
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
				Usage:   "run local sqs batch size",
				EnvVars: []string{"PREPALERT_BATCH_SIZE"},
				Value:   1,
			},
		},
		EnableBashCompletion: true,
		Version:              Version,
		Commands:             make([]*cli.Command, 0),
	}
	runCommand := &cli.Command{
		Name:      "run",
		Usage:     "run server (default command)",
		UsageText: "prepalert [global options] run [command options]",
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
	a.Action = func(ctx *cli.Context) error {
		return runCommand.Run(ctx)
	}
	a.Before = func(ctx *cli.Context) error {
		filter.SetMinLevel(logutils.LogLevel(strings.ToLower(ctx.String("log-level"))))
		runCommand.Flags = []cli.Flag{
			&cli.StringFlag{
				Name:  "address",
				Usage: "run address",
				Value: ctx.String("address"),
			},
			&cli.StringFlag{
				Name:  "prefix",
				Usage: "run server prefix",
				Value: ctx.String("prefix"),
			},
			&cli.StringFlag{
				Name:  "mode",
				Usage: "run mode",
				Value: ctx.String("mode"),
			},
			&cli.IntFlag{
				Name:  "batch-size",
				Usage: "run local sqs batch size",
				Value: ctx.Int("batch-size"),
			},
		}
		return nil
	}
	a.Commands = append(a.Commands, runCommand)
	sort.Sort(cli.FlagsByName(a.Flags))
	sort.Sort(cli.CommandsByName(a.Commands))
	for _, cmd := range a.Commands {
		sort.Sort(cli.FlagsByName(cmd.Flags))
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()
	if err := a.RunContext(ctx, os.Args); err != nil {
		log.Fatalf("[error] %v", err)
	}
}
