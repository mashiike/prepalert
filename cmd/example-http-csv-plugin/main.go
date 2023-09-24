package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/mashiike/prepalert/plugin"
	"github.com/mashiike/prepalert/provider"
)

type ProviderParameter struct {
	Endpoint string `json:"endpoint"`
}

type Provider struct {
	endpoints map[string]*url.URL
}

func (p *Provider) ValidateProviderParameter(ctx context.Context, pp *provider.ProviderParameter) error {
	slog.InfoContext(ctx, "call ValidateProviderParameter", "provider", pp.Name)
	if pp.Type != "http" {
		return fmt.Errorf("invalid provider type name %q: required plugin name http", pp.Type)
	}
	var cfg ProviderParameter
	if err := json.Unmarshal(pp.Params, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal provider params: %w", err)
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint: %w", err)
	}
	p.endpoints[pp.Name] = u
	return nil
}

func (p *Provider) GetQuerySchema(ctx context.Context) (*plugin.Schema, error) {
	slog.InfoContext(ctx, "call GetQuerySchema")
	return &plugin.Schema{
		Attributes: []plugin.AttributeSchema{
			{
				Name: "fields",
			},
			{
				Name: "limit",
			},
		},
	}, nil
}

type QueryParameter struct {
	Fields []string `json:"fields"`
	Limit  int      `json:"limit"`
}

func (p *Provider) RunQuery(ctx context.Context, req *plugin.RunQueryRequest) (*plugin.RunQueryResponse, error) {
	slog.InfoContext(ctx, "call RunQuery")
	var qp QueryParameter
	if err := json.Unmarshal(req.QueryParameters, &qp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query params: %w", err)
	}
	if len(qp.Fields) == 0 {
		qp.Fields = []string{"all"}
	}
	if qp.Limit <= 0 {
		qp.Limit = 100
	}

	endpoint, ok := p.endpoints[req.ProviderParameters.Name]
	if !ok {
		return nil, fmt.Errorf("provider name %q not found", req.ProviderParameters.Name)
	}
	c := *endpoint
	c.RawQuery = url.Values{
		"fields": []string{strings.Join(qp.Fields, ",")},
		"limit":  []string{fmt.Sprintf("%d", qp.Limit)},
	}.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Accept", "text/csv")
	httpReq.Header.Set("User-Agent", "prepalert-example-plugin")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to request: %s", resp.Status)
	}
	reader := csv.NewReader(resp.Body)
	reader.Comma = ','
	columns, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read csv: %w", err)
	}
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read csv: %w", err)
	}
	slog.InfoContext(ctx, "success to request", "columns", len(columns), "rows", len(rows))
	return &plugin.RunQueryResponse{
		Name:    req.QueryName,
		Query:   fmt.Sprintf("GET %s", c.String()),
		Columns: columns,
		Rows:    rows,
	}, nil
}

func main() {
	slog.SetDefault(slog.Default().With("plugin", "example-http-csv-plugin"))
	plugin.ServePlugin(plugin.WithProviderPlugin(&Provider{
		endpoints: make(map[string]*url.URL),
	}))
}
