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

func main() {
	plugin.ServePlugin(plugin.WithProviderPlugin(&Provider{}))
}
