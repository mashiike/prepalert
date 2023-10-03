package redshiftdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/mashiike/prepalert/provider"
	"github.com/mashiike/prepalert/provider/sqlprovider"
	redshiftdatasqldriver "github.com/mashiike/redshift-data-sql-driver"
)

type bypassLogger struct {
	Level slog.Level
	Impl  *slog.Logger
	once  sync.Once
}

func (l *bypassLogger) Printf(format string, v ...any) {
	l.once.Do(func() {
		l.Impl = slog.Default().With("component", "redshift-data-sql-driver")
	})
	l.Impl.Log(context.Background(), l.Level, fmt.Sprintf(format, v...))
}

func (l *bypassLogger) SetOutput(w io.Writer) {}
func (l *bypassLogger) Writer() io.Writer     { return io.Discard }

func init() {
	provider.RegisterProvider("redshift_data", NewProvider)
	redshiftdatasqldriver.SetLogger(&bypassLogger{Level: slog.LevelError})
	redshiftdatasqldriver.SetDebugLogger(&bypassLogger{Level: slog.LevelDebug})
}

type Provider struct {
	Type string
	Name string
	ProviderParameter
	DSN string
	*sqlprovider.Provider
}

type ProviderParameter struct {
	ClusterIdentifier *string `json:"cluster_identifier,omitempty"`
	Database          *string `json:"database,omitempty"`
	DbUser            *string `json:"db_user,omitempty"`
	WorkgroupName     *string `json:"workgroup_name,omitempty"`
	SecretsARN        *string `json:"secrets_arn,omitempty"`
	Timeout           int64   `json:"timeout,omitempty"`
	Polling           int64   `json:"polling_interval,omitempty"`
	Region            string  `json:"region,omitempty"`
}

func NewProvider(pp *provider.ProviderParameter) (*Provider, error) {
	p := &Provider{
		Type: pp.Type,
		Name: pp.Name,
		ProviderParameter: ProviderParameter{
			Region: os.Getenv("AWS_REGION"),
		},
	}
	if err := json.Unmarshal(pp.Params, &p.ProviderParameter); err != nil {
		return nil, fmt.Errorf("failed to parse params: %w", err)
	}
	cfg := &redshiftdatasqldriver.RedshiftDataConfig{
		ClusterIdentifier: p.ClusterIdentifier,
		Database:          p.Database,
		DbUser:            p.DbUser,
		WorkgroupName:     p.WorkgroupName,
		SecretsARN:        p.SecretsARN,
		Timeout:           time.Duration(p.Timeout) * time.Second,
		Polling:           time.Duration(p.Polling) * time.Second,
	}
	if p.ProviderParameter.Region != "" {
		cfg = cfg.WithRegion(p.ProviderParameter.Region)
	}
	p.DSN = cfg.String()
	sqlp, err := sqlprovider.NewProvider("redshift-data", p.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to create sql provider: %w", err)
	}
	p.Provider = sqlp
	return p, nil
}
