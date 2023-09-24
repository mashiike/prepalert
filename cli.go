package prepalert

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/alecthomas/kong"
)

type ErrorHandling int

const (
	ContinueOnError ErrorHandling = iota // if load config on error, continue run
	ReturnOnError                        // if load config on error, return error
)

func (e ErrorHandling) String() string {
	switch e {
	case ContinueOnError:
		return "continue"
	case ReturnOnError:
		return "return"
	default:
		return "unknown"
	}
}

func (e *ErrorHandling) UnmarshalText(text []byte) error {
	switch strings.ToLower(string(text)) {
	case "continue":
		*e = ContinueOnError
	case "return":
		*e = ReturnOnError
	default:
		return fmt.Errorf("unknown error handling: %s", string(text))
	}
	return nil
}

type CLI struct {
	LogLevel       string        `help:"output log-level" env:"PREPALERT_LOG_LEVEL" default:"info"`
	MackerelAPIKey string        `name:"mackerel-apikey" help:"for access mackerel API" env:"MACKEREL_APIKEY"`
	ErrorHandling  ErrorHandling `help:"error handling" env:"PREPALERT_ERROR_HANDLING" default:"continue" enum:"continue,return"`
	Config         string        `help:"config path" env:"PREPALERT_CONFIG" default:"."`
	Run            *RunOptions   `cmd:"" help:"run server (default command)" default:""`
	Init           struct{}      `cmd:"" help:"create initial config"`
	Validate       struct{}      `cmd:"" help:"validate the configuration"`
	Exec           *ExecOptions  `cmd:"" help:"Generate a virtual webhook from past alert to execute the rule"`
	Version        struct{}      `cmd:"" help:"Show version"`
}

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
	app := New(cli.MackerelAPIKey)
	switch cmd {
	case "init":
		return app.Init(ctx, cli.Config)
	case "version":
		fmt.Printf("prepalert %s\n", Version)
		return nil
	}
	slog.DebugContext(ctx, "load config", "config", cli.Config, "error_handling", cli.ErrorHandling)
	err = app.LoadConfig(cli.Config)
	if err != nil {
		slog.DebugContext(ctx, "load config failed", "error", err)
		if cli.ErrorHandling == ReturnOnError || cmd == "validate" {
			return err
		}
	}
	defer app.Close()
	switch cmd {
	case "run":
		return app.Run(ctx, cli.Run)
	case "validate":
		return nil
	case "exec":
		return app.Exec(ctx, cli.Exec)
	}
	return fmt.Errorf("unknown command: %s", cmd)
}
