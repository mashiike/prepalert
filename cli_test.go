package prepalert_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/mashiike/prepalert"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

func TestParseCLI(t *testing.T) {
	os.Setenv("MACKEREL_APIKEY", "*******************")
	os.Setenv("PREPALERT_CONFIG", "./testdata/")
	os.Setenv("PREPALERT_MODE", "worker")
	cases := []struct {
		args        []string
		expected    *prepalert.CLI
		cmd         string
		errStr      string
		checkOutput bool
	}{
		{
			args: []string{"prepalert"},
			cmd:  "run",
			expected: &prepalert.CLI{
				LogLevel:       "info",
				MackerelAPIKey: "*******************",
				Config:         "./testdata/",
				Run: &prepalert.RunOptions{
					Mode:      "worker",
					Address:   ":8080",
					Prefix:    "/",
					BatchSize: 1,
				},
				Exec: &prepalert.ExecOptions{},
			},
		},
		{
			args:        []string{"prepalert", "--help"},
			checkOutput: true,
		},
		{
			args: []string{"prepalert", "--config", ".", "run", "--mode", "http", "--log-level", "debug"},
			cmd:  "run",
			expected: &prepalert.CLI{
				LogLevel:       "debug",
				MackerelAPIKey: "*******************",
				Config:         ".",
				Run: &prepalert.RunOptions{
					Mode:      "http",
					Address:   ":8080",
					Prefix:    "/",
					BatchSize: 1,
				},
				Exec: &prepalert.ExecOptions{},
			},
		},
		{
			args: []string{"prepalert", "--config", ".", "init"},
			cmd:  "init",
			expected: &prepalert.CLI{
				LogLevel:       "info",
				MackerelAPIKey: "*******************",
				Config:         ".",
				Run: &prepalert.RunOptions{
					Mode:      "worker",
					Address:   ":8080",
					Prefix:    "/",
					BatchSize: 1,
				},
				Exec: &prepalert.ExecOptions{},
			},
		},
		{
			args:   []string{"prepalert", "--config", ".", "exec"},
			cmd:    "exec",
			errStr: `expected "<alert-id>"`,
		},
		{
			args:        []string{"prepalert", "--config", ".", "exec", "--help"},
			cmd:         "",
			checkOutput: true,
		},
		{
			args: []string{"prepalert", "--config", ".", "exec", "--log-level", "debug", "xxxxxxxx"},
			cmd:  "exec",
			expected: &prepalert.CLI{
				LogLevel:       "debug",
				MackerelAPIKey: "*******************",
				Config:         ".",
				Run: &prepalert.RunOptions{
					Mode:      "worker",
					Address:   ":8080",
					Prefix:    "/",
					BatchSize: 1,
				},
				Exec: &prepalert.ExecOptions{
					AlertID: "xxxxxxxx",
				},
			},
		},
		{
			args: []string{"prepalert", "validate", "--config", ".", "--log-level", "debug"},
			cmd:  "validate",
			expected: &prepalert.CLI{
				LogLevel:       "debug",
				MackerelAPIKey: "*******************",
				Config:         ".",
				Run: &prepalert.RunOptions{
					Mode:      "worker",
					Address:   ":8080",
					Prefix:    "/",
					BatchSize: 1,
				},
				Exec: &prepalert.ExecOptions{},
			},
		},
		{
			args: []string{"prepalert", "version", "--config", ".", "--log-level", "debug"},
			cmd:  "version",
			expected: &prepalert.CLI{
				LogLevel:       "debug",
				MackerelAPIKey: "*******************",
				Config:         ".",
				Run: &prepalert.RunOptions{
					Mode:      "worker",
					Address:   ":8080",
					Prefix:    "/",
					BatchSize: 1,
				},
				Exec: &prepalert.ExecOptions{},
			},
		},
	}

	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
	type bailout struct{}
	for _, c := range cases {
		casename := strings.Join(c.args, "_")
		t.Run(casename, func(t *testing.T) {
			var output bytes.Buffer
			cmd, cli, err := func() (string, *prepalert.CLI, error) {
				defer func() {
					err := recover()
					if _, ok := err.(bailout); err != nil && !ok {
						panic(err)
					}
				}()
				return prepalert.ParseCLI(
					context.Background(),
					c.args[1:],
					kong.Writers(&output, &output),
					kong.Exit(func(i int) { panic(bailout{}) }),
				)
			}()

			if c.errStr != "" {
				require.EqualError(t, err, c.errStr)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.cmd, cmd)
				require.EqualValues(t, c.expected, cli)
			}
			if c.checkOutput {
				name := strings.ReplaceAll(casename, "--", "_")
				name = strings.ReplaceAll(name, ".", "")
				name = strings.ReplaceAll(name, "__", "_")
				name = strings.ReplaceAll(name, "_help", "")
				g.Assert(t, "parse_cli__"+name, output.Bytes())
			}
		})
	}
}
