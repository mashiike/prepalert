package main

import (
	"context"
	"errors"
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
	"github.com/mashiike/prepalert/hclconfig"
	_ "github.com/mashiike/prepalert/queryrunner/cloudwatchlogsinsights"
	_ "github.com/mashiike/prepalert/queryrunner/redshiftdata"
	_ "github.com/mashiike/prepalert/queryrunner/s3select"
	"github.com/mashiike/prepalert/wizard"
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
				Usage:   "config path",
				EnvVars: []string{"CONFIG", "PREPALERT_CONFIG"},
				Value:   ".",
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
				Value:   "http",
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
		Commands: []*cli.Command{
			{
				Name:      "init",
				Usage:     "create inital config",
				UsageText: "prepalert init\nprepalert [global options] init",
				Action: func(ctx *cli.Context) error {
					return wizard.Run(ctx.Context, Version, ctx.String("mackerel-apikey"), ctx.String("config"))
				},
			},
			{
				Name:      "validate",
				Usage:     "validate the configuration",
				UsageText: "prepalert [global options] validate",
				Action: func(ctx *cli.Context) error {
					_, err := hclconfig.Load(ctx.String("config"), Version)
					return err
				},
			},
			{
				Name:      "exec",
				Usage:     "Generate a virtual webhook from past alert to execute the rule",
				UsageText: "prepalert [global options] exec <alert_id>",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() == 0 {
						return errors.New("alert_id is required")
					}
					cfg, err := hclconfig.Load(ctx.String("config"), Version)
					if err != nil {
						return err
					}
					app, err := prepalert.New(ctx.String("mackerel-apikey"), cfg)
					if err != nil {
						return err
					}
					return app.Exec(ctx.Context, ctx.Args().First())
				},
			},
		},
	}
	runCommand := &cli.Command{
		Name:      "run",
		Usage:     "run server (default command)",
		UsageText: "prepalert [global options] run [command options]",
		Action: func(ctx *cli.Context) error {
			cfg, err := hclconfig.Load(ctx.String("config"), Version)
			if err != nil {
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
