package plugin

import (
	"context"
	"encoding/json"
	"errors"

	plugin "github.com/hashicorp/go-plugin"
	paproto "github.com/mashiike/prepalert/plugin/proto"
	"github.com/mashiike/prepalert/provider"
	"google.golang.org/grpc"
)

// type GetQuerySchemaRequest struct{}
// type GetQuerySchemaResponse struct{}
// type RunQueryRequest struct{}
// type RunQueryResponse struct{}
type Provider interface {
	ValidateProviderParameter(ctx context.Context, pp *provider.ProviderParameter) error
	// GetQuerySchema(ctx context.Context, req *GetQuerySchemaRequest) (*GetQuerySchemaResponse, error)
	// RunQuery(ctx context.Context, req *RunQueryRequest) (*RunQueryResponse, error)
}

// Here is the gRPC server that GRPCClient talks to.
type GRPCServer struct {
	// This is the real implementation
	Impl Provider
	paproto.UnimplementedProviderServer
}

func (m *GRPCServer) ValidateProviderParameter(ctx context.Context, r *paproto.ValidatProviderPaameter_Request) (*paproto.ValidatProviderPaameter_Response, error) {
	params := r.GetParameter()
	err := m.Impl.ValidateProviderParameter(
		ctx,
		&provider.ProviderParameter{
			Type:   params.GetType(),
			Name:   params.GetName(),
			Params: json.RawMessage(params.GetJson()),
		},
	)
	if err != nil {
		return &paproto.ValidatProviderPaameter_Response{
			Ok:      false,
			Message: err.Error(),
		}, nil
	}
	return &paproto.ValidatProviderPaameter_Response{
		Ok: true,
	}, nil
}

func (m *GRPCServer) GetQuerySchema(context.Context, *paproto.GetQuerySchema_Request) (*paproto.GetQuerySchema_Response, error) {
	return nil, errors.New("not implemeted yet")

}
func (m *GRPCServer) RunQuery(context.Context, *paproto.RunQuery_Request) (*paproto.RunQuery_Response, error) {
	return nil, errors.New("not implemeted yet")
}

type GRPCClient struct{ client paproto.ProviderClient }

func (m *GRPCClient) ValidateProviderParameter(ctx context.Context, pp *provider.ProviderParameter) error {
	r := &paproto.ValidatProviderPaameter_Request{
		Parameter: &paproto.ProviderParameter{
			Type: pp.Type,
			Name: pp.Name,
			Json: string(pp.Params),
		},
	}
	res, err := m.client.ValidateProviderParameter(ctx, r)
	if err != nil {
		return err
	}
	if !res.Ok {
		return errors.New(res.Message)
	}
	return nil
}

type ProviderPlugin struct {
	plugin.Plugin
	Impl Provider
}

func (p *ProviderPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	paproto.RegisterProviderServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *ProviderPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: paproto.NewProviderClient(c)}, nil
}
