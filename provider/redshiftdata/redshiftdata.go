package redshiftdata

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/provider/sqlprovider"
	redshiftdatasqldriver "github.com/mashiike/redshift-data-sql-driver"
)

func init() {
	prepalert.RegisterProvider("redshift_data", NewProvider)
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
}

func NewProvider(pp *prepalert.ProviderParameter) (*Provider, error) {
	p := &Provider{
		Type: pp.Type,
		Name: pp.Name,
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
	p.DSN = cfg.String()
	sqlp, err := sqlprovider.NewProvider("redshift-data", p.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to create sql provider: %w", err)
	}
	p.Provider = sqlp
	return p, nil
}
