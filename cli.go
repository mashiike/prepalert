package prepalert

import (
	"context"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
)

type CLI struct {
	LogLevel       string       `help:"output log-level" env:"PREPALERT_LOG_LEVEL" default:"info"`
	MackerelAPIKey string       `name:"mackerel-apikey" help:"for access mackerel API" env:"MACKEREL_APIKEY"`
	Config         string       `help:"config path" env:"PREPALERT_CONFIG" default:"."`
	Run            *RunOptions  `cmd:"" help:"run server (default command)" default:""`
	Init           struct{}     `cmd:"" help:"create inital config"`
	Validate       struct{}     `cmd:"" help:"validate the configuration"`
	Exec           *ExecOptions `cmd:"" help:"Generate a virtual webhook from past alert to execute the rule"`
	Version        struct{}     `cmd:"" help:"Show version"`
}

type ExecOptions struct {
	AlertID string `arg:"" name:"alert-id" help:"Mackerel AlertID" required:""`
}

var Version = "current"

func ParseCLI(ctx context.Context, args []string, opts ...kong.Option) (string, *CLI, error) {
	var cli CLI
	cliOptions := []kong.Option{
		kong.Vars{"version": Version},
		kong.Name("prepalert"),
		kong.Description("A webhook server for prepare alert memo"),
	}
	cliOptions = append(cliOptions, opts...)
	parser, err := kong.New(&cli, cliOptions...)
	if err != nil {
		return "", nil, err
	}
	kctx, err := parser.Parse(args)
	if err != nil {
		return "", nil, err
	}
	cmd := strings.Fields(kctx.Command())[0]
	return cmd, &cli, nil
}

func RunCLI(ctx context.Context, args []string, setLogLevel func(string)) error {
	cmd, cli, err := ParseCLI(ctx, args)
	if err != nil {
		return err
	}
	setLogLevel(cli.LogLevel)
	app, err := New(cli.MackerelAPIKey)
	if err != nil {
		return err
	}
	switch cmd {
	case "init":
		return app.Init(ctx, cli.Config)
	case "version":
		fmt.Printf("prepalert %s\n", Version)
		return nil
	}
	err = app.LoadConfig(cli.Config)
	if err != nil {
		return err
	}
	defer app.Close()
	switch cmd {
	case "run":
		return app.Run(ctx, cli.Run)
	case "validate":
		return nil
	case "exec":
		return app.Exec(ctx, cli.Exec.AlertID)
	}
	return fmt.Errorf("unknown command: %s", cmd)
}
