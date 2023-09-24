package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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

func NewClient(pluginName string, cmd string, sync bool) *Client {
	var stderr, stdout io.Writer = io.Discard, io.Discard
	if sync {
		stderr, stdout = os.Stderr, os.Stdout
	}
	client := plugin.NewClient(
		&plugin.ClientConfig{
			HandshakeConfig: Handshake,
			Plugins:         PluginMap,
			Cmd:             exec.Command("sh", "-c", cmd),
			AllowedProtocols: []plugin.Protocol{
				plugin.ProtocolGRPC,
			},
			Stderr:     stderr,
			SyncStderr: stderr,
			SyncStdout: stdout,
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

func NewRemoteProviderFactory(pluginName string, cmd string, sync bool) (*RemoteProviderFactory, func() error, error) {
	c := NewClient(pluginName, cmd, sync)
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
	name   string
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
		name:   name,
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
		key := block.Type
		if len(block.Labels) > 0 {
			key += "." + strings.Join(block.Labels, ".")
		}
		b.NestedBlocks[key] = &decodedBlock{
			Block: block,
		}
		diags = diags.Extend(b.NestedBlocks[key].DecodeBody(block.Body, evalCtx, nestedBlockSchemaByType[block.Type]))
	}
	return diags
}

func (b *decodedBlock) toJSON(evalCtx *hcl.EvalContext) (map[string]interface{}, error) {
	params := make(map[string]interface{}, len(b.Attributes)+len(b.NestedBlocks))
	for _, attr := range b.Attributes {
		value, err := attr.Expr.Value(evalCtx)
		if err != nil {
			return nil, fmt.Errorf("eval attribute error: %w", err)
		}
		var jsonValue json.RawMessage
		if err := hclutil.UnmarshalCTYValue(value, &jsonValue); err != nil {
			return nil, fmt.Errorf("unmarshal attribute error: %w", err)
		}
		params[attr.Name] = jsonValue
	}
	blocks := make(map[string]interface{}, len(b.NestedBlocks))
	for key, block := range b.NestedBlocks {
		value, err := block.toJSON(evalCtx)
		if err != nil {
			return nil, fmt.Errorf("convert nested block error: %w", err)
		}
		blocks[key] = value
	}
	// split . and merge into params map
	for key, value := range blocks {
		parts := strings.Split(key, ".")
		current := params
		for _, part := range parts[:len(parts)-1] {
			if _, ok := current[part]; !ok {
				current[part] = make(map[string]interface{})
			}
			current = current[part].(map[string]interface{})
		}
		current[parts[len(parts)-1]] = value
	}
	return params, nil
}

func (b *decodedBlock) ToJSON(evalCtx *hcl.EvalContext) ([]byte, error) {
	params, err := b.toJSON(evalCtx)
	if err != nil {
		return nil, err
	}
	bs, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal json error: %w", err)
	}
	return bs, nil
}

func (rq *RemoteQuery) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*provider.QueryResult, error) {
	bs, err := rq.params.ToJSON(evalCtx)
	if err != nil {
		return nil, fmt.Errorf("convert query body to json error: %w", err)
	}
	req := &RunQueryRequest{
		ProviderParameters: rq.rp.pp,
		QueryName:          rq.name,
		QueryParameters:    json.RawMessage(bs),
	}
	resp, err := rq.rp.impl.RunQuery(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("impl.RunQuery: %w", err)
	}
	params := make([]interface{}, len(resp.Params))
	for i, param := range resp.Params {
		var value interface{}
		if err := json.Unmarshal(param, &value); err != nil {
			return nil, fmt.Errorf("unmarshal query result param error: %w", err)
		}
		params[i] = value
	}
	if resp.Columns != nil {
		rows := make([][]json.RawMessage, len(resp.Rows))
		for i, row := range resp.Rows {
			rows[i] = make([]json.RawMessage, len(row))
			for j, val := range row {
				if json.Valid([]byte(val)) {
					rows[i][j] = json.RawMessage(val)
				} else {
					rows[i][j] = json.RawMessage(fmt.Sprintf("%q", val))
				}
			}
		}
		return provider.NewQueryResult(resp.Name, resp.Query, params, resp.Columns, rows), nil
	}
	lines := make([]map[string]json.RawMessage, len(resp.JSONLines))
	for i, jl := range resp.JSONLines {
		var line map[string]json.RawMessage
		if err := json.Unmarshal(jl, &line); err != nil {
			return nil, fmt.Errorf("unmarshal query result line error: %w", err)
		}
		lines[i] = line
	}
	qr := provider.NewQueryResultWithJSONLines(resp.Name, resp.Query, params, lines...)
	return qr, nil
}
