package main

import (
	"context"
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

func main() {
	plugin.ServePlugin(plugin.WithProviderPlugin(&Provider{}))
}
