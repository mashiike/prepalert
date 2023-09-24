package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mashiike/prepalert/plugin"
	"github.com/mashiike/prepalert/provider"
)

type Provider struct{}

func (p *Provider) ValidateProviderParameter(ctx context.Context, pp *provider.ProviderParameter) error {
	log.Println(pp.String())
	return nil
}

func (p *Provider) GetQuerySchema(ctx context.Context) (*plugin.Schema, error) {
	return &plugin.Schema{
		Attributes: []plugin.AttributeSchema{
			{
				Name:     "code",
				Required: true,
			},
		},
		Blocks: []plugin.BlockSchema{
			{
				Type:         "details",
				Required:     true,
				UniqueLabels: true,
				Body: &plugin.Schema{
					Attributes: []plugin.AttributeSchema{
						{
							Name:     "description",
							Required: true,
						},
					},
				},
			},
		},
	}, nil
}

func (p *Provider) RunQuery(ctx context.Context, req *plugin.RunQueryRequest) (*plugin.RunQueryResponse, error) {
	return &plugin.RunQueryResponse{
		Name: "test",
		JSONLines: []json.RawMessage{
			[]byte(`{"system":"app", "code":"hoge", "description":"hoge is test app error code"}`),
			[]byte(`{"system":"app", "code":"fuga", "description":"fuga is test app error code"}`),
		},
	}, nil
}

func main() {
	plugin.ServePlugin(plugin.WithProviderPlugin(&Provider{}))
}
