package plugin

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert/provider"
)

var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "PREPALERT_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "prepalert-grpc-plugin-protocol",
}

var PluginMap = map[string]plugin.Plugin{
	"provider": &ProviderPlugin{},
}

type serveOption struct {
	plugins map[string]plugin.Plugin
}

func WithProviderPlugin(impl Provider) func(*serveOption) {
	return func(opt *serveOption) {
		if impl != nil {
			opt.plugins["provider"] = &ProviderPlugin{Impl: impl}
		}
	}
}

func ServePlugin(optFns ...func(*serveOption)) error {
	opt := serveOption{
		plugins: make(map[string]plugin.Plugin),
	}
	for _, optFn := range optFns {
		optFn(&opt)
	}
	if len(opt.plugins) == 0 {
		return errors.New("no implement plugins")
	}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         opt.plugins,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
	return nil
}

type Client struct {
	pluginName string
	impl       *plugin.Client
	once       sync.Once
	closeMu    sync.Mutex
	closed     bool
	rpcClient  plugin.ClientProtocol
	initErr    error
}

func NewClient(pluginName string, cmd string) *Client {
	client := plugin.NewClient(
		&plugin.ClientConfig{
			HandshakeConfig: Handshake,
			Plugins:         PluginMap,
			Cmd:             exec.Command("sh", "-c", cmd),
			AllowedProtocols: []plugin.Protocol{
				plugin.ProtocolGRPC,
			},
			Logger: hclog.New(&hclog.LoggerOptions{
				Output: hclog.DefaultOutput,
				Level:  hclog.Error,
				Name:   pluginName,
			}),
		},
	)
	ret := &Client{pluginName: pluginName, impl: client}
	runtime.SetFinalizer(client, func(c *plugin.Client) {
		c.Kill()
	})
	return ret
}

func (c *Client) init() error {
	c.once.Do(func() {
		c.rpcClient, c.initErr = c.impl.Client()
	})
	return c.initErr
}

func (c *Client) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.impl.Kill()
	c.impl = nil
	err := c.rpcClient.Close()
	c.rpcClient = nil
	return err
}

func (c *Client) NewProviderService() (Provider, error) {
	if err := c.init(); err != nil {
		return nil, fmt.Errorf("init client error: %w", err)
	}
	raw, err := c.rpcClient.Dispense("provider")
	if err != nil {
		return nil, fmt.Errorf("dispense provider error: %w", err)
	}
	p, ok := raw.(Provider)
	if !ok {
		return nil, fmt.Errorf("cannot convert to Provider")
	}
	return p, nil
}

type RemoteProviderFactory struct {
	pluginName string
	provier    Provider
}

func NewRemoteProviderFactory(pluginName string, cmd string) (*RemoteProviderFactory, func() error, error) {
	c := NewClient(pluginName, cmd)
	f := &RemoteProviderFactory{
		pluginName: pluginName,
	}
	p, err := c.NewProviderService()
	if err != nil {
		return nil, c.Close, err
	}
	f.provier = p
	return f, c.Close, err
}

type RemoteProvider struct {
	impl        Provider
	pp          *provider.ProviderParameter
	querySchema *Schema
}

type RemoteQuery struct {
	rp     *RemoteProvider
	params *decodedBlock
}

func (f *RemoteProviderFactory) NewProvider(pp *provider.ProviderParameter) (*RemoteProvider, error) {
	rp := &RemoteProvider{
		impl: f.provier,
		pp:   pp,
	}
	err := rp.impl.ValidateProviderParameter(context.Background(), pp)
	if err != nil {
		return nil, fmt.Errorf("validate provider parameter error: %w", err)
	}
	rp.querySchema, err = rp.impl.GetQuerySchema(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get query schema error: %w", err)
	}
	return rp, nil
}

func (rp *RemoteProvider) NewQuery(name string, body hcl.Body, evalCtx *hcl.EvalContext) (provider.Query, error) {
	rq := &RemoteQuery{
		rp:     rp,
		params: &decodedBlock{},
	}
	diags := rq.params.DecodeBody(body, evalCtx, rp.querySchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decode query body error: %w", diags)
	}
	return rq, nil
}

type decodedBlock struct {
	Attributes hcl.Attributes
	*hcl.Block
	NestedBlocks map[string]*decodedBlock
}

func (b *decodedBlock) DecodeBody(body hcl.Body, evalCtx *hcl.EvalContext, schema *Schema) hcl.Diagnostics {
	s := &hcl.BodySchema{
		Attributes: make([]hcl.AttributeSchema, 0, len(schema.Attributes)),
		Blocks:     make([]hcl.BlockHeaderSchema, 0, len(schema.Blocks)),
	}
	for _, attr := range schema.Attributes {
		s.Attributes = append(s.Attributes, hcl.AttributeSchema{
			Name:     attr.Name,
			Required: attr.Required,
		})
	}
	nestedBlockSchemaByType := make(map[string]*Schema)
	restriction := make([]hclutil.BlockRestrictionSchema, 0, len(schema.Blocks))
	for _, block := range schema.Blocks {
		s.Blocks = append(s.Blocks, hcl.BlockHeaderSchema{
			Type:       block.Type,
			LabelNames: block.LabelNames,
		})
		nestedBlockSchemaByType[block.Type] = block.Body
		restriction = append(restriction, hclutil.BlockRestrictionSchema{
			Type:         block.Type,
			Required:     block.Required,
			Unique:       block.Unique,
			UniqueLabels: block.UniqueLabels,
		})
	}
	content, diags := body.Content(s)
	if diags.HasErrors() {
		return diags
	}
	diags = diags.Extend(hclutil.RestrictBlock(content, restriction...))
	if diags.HasErrors() {
		return diags
	}
	b.Attributes = content.Attributes
	b.NestedBlocks = make(map[string]*decodedBlock)
	for _, block := range content.Blocks {
		key := block.Type + "." + strings.Join(block.Labels, ".")
		b.NestedBlocks[key] = &decodedBlock{
			Block: block,
		}
		diags = diags.Extend(b.NestedBlocks[key].DecodeBody(block.Body, evalCtx, nestedBlockSchemaByType[block.Type]))
	}
	return diags
}

func (rq *RemoteQuery) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*provider.QueryResult, error) {
	return nil, errors.New("not implemented yet")
}
