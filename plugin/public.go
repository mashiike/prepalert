package plugin

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"

	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/hcl/v2"
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
	impl Provider
	pp   *provider.ProviderParameter
}

func (f *RemoteProviderFactory) NewProvider(pp *provider.ProviderParameter) (*RemoteProvider, error) {
	rp := &RemoteProvider{
		impl: f.provier,
		pp:   pp,
	}
	err := rp.impl.ValidateProviderParameter(context.Background(), pp)
	if err != nil {
		return nil, err
	}
	return rp, nil
}

func (rp *RemoteProvider) NewQuery(name string, body hcl.Body, evalCtx *hcl.EvalContext) (provider.Query, error) {
	return nil, errors.New("not implemented yet")
}
