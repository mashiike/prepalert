package prepalert

import (
	"fmt"
)

type QueryRunner interface {
	Compile(*QueryConfig) (CompiledQuery, error)
}

func NewQueryRunner(cfg *QueryRunnerConfig) (QueryRunner, error) {
	switch cfg.Type {
	case QueryRunnerTypeRedshiftData:
		return newRedshiftDataQueryRunner(cfg)
	default:
		return nil, fmt.Errorf("unknwon query runner type = %d", cfg.Type)
	}
}

type QueryRunners map[string]QueryRunner

func NewQueryRunners(cfgs QueryRunnerConfigs) (QueryRunners, error) {
	runners := make(QueryRunners, cfgs.Len())
	for _, cfg := range cfgs {
		runner, err := NewQueryRunner(cfg)
		if err != nil {
			return nil, fmt.Errorf("new query runner `%s`:%w", cfg.Name, err)
		}
		runners[cfg.Name] = runner
	}
	return runners, nil
}

func (runners QueryRunners) Get(name string) (QueryRunner, bool) {
	runner, ok := runners[name]
	return runner, ok
}
