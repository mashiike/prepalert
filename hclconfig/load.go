package hclconfig

import (
	"github.com/mashiike/hclconfig"
	"github.com/zclconf/go-cty/cty"
)

func Load(path string, version string, optFns ...func(loader *hclconfig.Loader)) (*Config, error) {
	loader := hclconfig.New()
	for _, optFn := range optFns {
		optFn(loader)
	}
	loader.Variables(map[string]cty.Value{
		"var": cty.ObjectVal(map[string]cty.Value{
			"version": cty.StringVal(version),
		}),
	})
	cfg := &Config{}
	if err := loader.Load(cfg, path); err != nil {
		return nil, err
	}
	return cfg, nil
}
