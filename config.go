package prepalert

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	gv "github.com/hashicorp/go-version"
	gc "github.com/kayac/go-config"
)

type Config struct {
	RequiredVersion string             `yaml:"required_version,omitempty"`
	Service         string             `yaml:"service,omitempty"`
	SQSQueueName    string             `yaml:"sqs_queue_name,omitempty"`
	Auth            *AuthConfig        `yaml:"auth,omitempty"`
	QueryRunners    QueryRunnerConfigs `yaml:"query_runners,omitempty"`
	Rules           []*RuleConfig      `yaml:"rules,omitempty"`

	configFilePath     string         `yaml:"-,omitempty"`
	versionConstraints gv.Constraints `yaml:"-,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{}
}

type AuthConfig struct {
	ClientID     string `yaml:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret,omitempty"`
}

//go:generate go-enum -type=QueryRunnerType -yaml -trimprefix QueryRunnerType
type QueryRunnerType int

const (
	QueryRunnerTypeRedshiftData QueryRunnerType = iota + 1
)

type QueryRunnerConfigs []*QueryRunnerConfig

func (cfgs QueryRunnerConfigs) Get(name string) (*QueryRunnerConfig, bool) {
	for _, cfg := range cfgs {
		if cfg.Name == name {
			return cfg, true
		}
	}
	return nil, false
}

func (cfgs QueryRunnerConfigs) Len() int {
	return len(cfgs)
}

type QueryRunnerConfig struct {
	Name string          `yaml:"name,omitempty"`
	Type QueryRunnerType `yaml:"type,omitempty"`

	//For RedshiftData
	ClusterIdentifier string `yaml:"cluster_identifier,omitempty"`
	Database          string `yaml:"database,omitempty"`
	DBUser            string `yaml:"db_user,omitempty"`
	WorkgroupName     string `yaml:"workgroup_name,omitempty"`
	SecretsARN        string `yaml:"secrets_arn,omitempty"`
}

type RuleConfig struct {
	Monitor *MonitorConfig `yaml:"monitor,omitempty"`
	Queries []*QueryConfig `yaml:"queries,omitempty"`
	Memo    *MemoConfig    `yaml:"memo,omitempty"`
}

type MonitorConfig struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

type QueryConfig struct {
	Name   string `yaml:"name,omitempty"`
	Runner string `yaml:"runner,omitempty"`
	File   string `yaml:"file,omitempty"`
	Query  string `yaml:"query,omitempty"`
}

type MemoConfig struct {
	File string `yaml:"file,omitempty"`
	Text string `yaml:"text,omitempty"`
}

func (cfg *Config) Load(path string) error {
	if err := gc.LoadWithEnv(cfg, path); err != nil {
		return err
	}
	cfg.configFilePath = filepath.Dir(path)
	return cfg.Restrict()
}

func (cfg *Config) ValidateVersion(version string) error {
	if cfg.versionConstraints == nil {
		log.Println("[warn] required_version is empty. Skip checking required_version.")
		return nil
	}
	versionParts := strings.SplitN(version, "-", 2)
	v, err := gv.NewVersion(versionParts[0])
	if err != nil {
		log.Printf("[warn]: Invalid version format \"%s\". Skip checking required_version.", version)
		// invalid version string (e.g. "current") always allowed
		return nil
	}
	if !cfg.versionConstraints.Check(v) {
		return fmt.Errorf("version %s does not satisfy constraints required_version: %s", version, cfg.versionConstraints)
	}
	return nil
}

func (cfg *Config) Restrict() error {
	if cfg.RequiredVersion != "" {
		constraints, err := gv.NewConstraint(cfg.RequiredVersion)
		if err != nil {
			return fmt.Errorf("required_version has invalid format: %w", err)
		}
		cfg.versionConstraints = constraints
	}
	if cfg.Service == "" {
		return errors.New("service is required")
	}
	if cfg.SQSQueueName == "" {
		return errors.New("sqs_queue_name is required")
	}
	if cfg.Auth != nil {
		if err := cfg.Auth.Restrict(); err != nil {
			return fmt.Errorf("auth:%w", err)
		}
	}
	if err := cfg.QueryRunners.Restrict(); err != nil {
		return fmt.Errorf("query_runners%w", err)
	}

	for i, rule := range cfg.Rules {
		if err := rule.Restrict(cfg.configFilePath, cfg.QueryRunners); err != nil {
			return fmt.Errorf("rules[%d]:%w", i, err)
		}
	}
	return nil
}

func (cfg *AuthConfig) Restrict() error {
	if cfg.ClientID == "" {
		return errors.New("client_id is required")
	}
	if cfg.ClientSecret == "" {
		return errors.New("client_secret is required")
	}
	return nil
}

func (cfgs QueryRunnerConfigs) Restrict() error {
	for i, cfg := range cfgs {
		if err := cfg.Restrict(); err != nil {
			return fmt.Errorf("[%d]:%w", i, err)
		}
	}
	return nil
}

func (cfg *QueryRunnerConfig) Restrict() error {
	if cfg.Name == "" {
		return errors.New("name is required")
	}
	if !cfg.Type.Registered() {
		return errors.New("invalid type")
	}
	switch cfg.Type {
	case QueryRunnerTypeRedshiftData:
		if cfg.SecretsARN != "" {
			return nil
		}
		if cfg.ClusterIdentifier == "" {
			return errors.New("if type is RedshiftData and secrets_arn is empty, cluster_identifier is required")
		}
		if cfg.DBUser == "" {
			return errors.New("if type is RedshiftData and secrets_arn is empty, db_user is required")
		}
		if cfg.Database == "" {
			return errors.New("if type is RedshiftData and secrets_arn is empty, database is required")
		}
		return nil
	default:
		return errors.New("unknown type")
	}
}

func (cfg *RuleConfig) Restrict(baseDir string, queryRunners QueryRunnerConfigs) error {
	if cfg.Monitor == nil {
		return errors.New("monitor is required")
	}
	if err := cfg.Monitor.Restrict(); err != nil {
		return fmt.Errorf("monitor.%w", err)
	}
	for i, query := range cfg.Queries {
		if err := query.Restrict(baseDir, queryRunners); err != nil {
			return fmt.Errorf("queries[%d].%w", i, err)
		}
	}
	if cfg.Memo == nil {
		return errors.New("memo is required")
	}
	if err := cfg.Memo.Restrict(baseDir); err != nil {
		return fmt.Errorf("memo.%w", err)
	}
	return nil
}

func (cfg *MonitorConfig) Restrict() error {
	if cfg.ID == "" && cfg.Name == "" {
		return errors.New("either id or name is required")
	}
	return nil
}

func (cfg *QueryConfig) Restrict(baseDir string, queryRunners QueryRunnerConfigs) error {
	if cfg.Name == "" {
		return errors.New("name is required")
	}
	if cfg.Runner == "" {
		return errors.New("runner is required")
	}
	if _, ok := queryRunners.Get(cfg.Runner); !ok {
		return fmt.Errorf("runner `%s` not found", cfg.Runner)
	}
	if cfg.Query != "" {
		return nil
	}
	if cfg.File == "" {
		return errors.New("either file or query is required")
	}
	path := cfg.File
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, cfg.File)
	}
	fp, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("file open:%w", err)
	}
	defer fp.Close()
	bs, err := io.ReadAll(fp)
	if err != nil {
		return fmt.Errorf("file read:%w", err)
	}
	cfg.Query = string(bs)
	return nil
}

func (cfg *MemoConfig) Restrict(baseDir string) error {
	if cfg.Text != "" {
		return nil
	}
	if cfg.File == "" {
		return errors.New("either file or text is required")
	}
	path := cfg.File
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, cfg.File)
	}
	fp, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("file open:%w", err)
	}
	defer fp.Close()
	bs, err := io.ReadAll(fp)
	if err != nil {
		return fmt.Errorf("file read:%w", err)
	}
	cfg.Text = string(bs)
	return nil
}
