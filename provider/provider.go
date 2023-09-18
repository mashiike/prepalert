package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclutil"
	"github.com/zclconf/go-cty/cty"
)

//go:generate mockgen -source=$GOFILE -destination=../mock/mock_$GOFILE -package=mock

type ProviderParameter struct {
	Type      string
	Name      string
	Params    json.RawMessage
	ctyParams map[string]cty.Value // same as Params but in cty.Value, for MarshalCTYValue
}

func (p *ProviderParameter) String() string {
	return fmt.Sprintf("%s.%s", p.Type, p.Name)
}

func (p *ProviderParameter) SetParams(params map[string]cty.Value) error {
	p.ctyParams = params
	if err := hclutil.UnmarshalCTYValue(cty.ObjectVal(p.ctyParams), &p.Params); err != nil {
		return err
	}
	return nil
}

type Query interface {
	Run(ctx context.Context, evalCtx *hcl.EvalContext) (*QueryResult, error)
}

type Provider interface {
	NewQuery(name string, body hcl.Body, evalCtx *hcl.EvalContext) (Query, error)
}

type ProviderFactory func(*ProviderParameter) (Provider, error)
type GenericProviderFactory[T Provider] func(*ProviderParameter) (T, error)

var (
	providersMu       sync.RWMutex
	providerFactories = map[string]ProviderFactory{}
)

func RegisterProvider[T Provider](typeName string, factory GenericProviderFactory[T]) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providerFactories[typeName] = func(pp *ProviderParameter) (Provider, error) {
		return factory(pp)
	}
}

func UnregisterProvider(typeName string) {
	providersMu.Lock()
	defer providersMu.Unlock()
	delete(providerFactories, typeName)
}

func NewProvider(param *ProviderParameter) (Provider, error) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	factory, ok := providerFactories[param.Type]
	if !ok {
		return nil, fmt.Errorf("unknown provider type %q", param.Type)
	}
	return factory(param)
}

type ProviderParameters []*ProviderParameter

func (p ProviderParameters) MarshalCTYValue() (cty.Value, error) {
	values := make(map[string]map[string]cty.Value)
	for _, provider := range p {
		if values[provider.Type] == nil {
			values[provider.Type] = make(map[string]cty.Value)
		}
		values[provider.Type][provider.Name] = cty.ObjectVal(provider.ctyParams)
	}
	types := make(map[string]cty.Value)
	for k, v := range values {
		types[k] = cty.ObjectVal(v)
	}
	return cty.ObjectVal(types), nil
}
