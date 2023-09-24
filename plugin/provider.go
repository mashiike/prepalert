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

type Schema struct {
	Attributes []AttributeSchema
	Blocks     []BlockSchema
}

type AttributeSchema struct {
	Name     string
	Required bool
}

type BlockSchema struct {
	Type         string
	LabelNames   []string
	Unique       bool
	Required     bool
	UniqueLabels bool
	Body         *Schema
}

type Provider interface {
	ValidateProviderParameter(ctx context.Context, pp *provider.ProviderParameter) error
	GetQuerySchema(ctx context.Context) (*Schema, error)
	// RunQuery(ctx context.Context, req *RunQueryRequest) (*RunQueryResponse, error)
}

func NewSchemaWithProto(s *paproto.Schema) *Schema {
	res := &Schema{
		Attributes: make([]AttributeSchema, len(s.Attributes)),
		Blocks:     make([]BlockSchema, len(s.Blocks)),
	}
	for i, attr := range s.Attributes {
		res.Attributes[i] = AttributeSchema{
			Name:     attr.GetName(),
			Required: attr.GetRequired(),
		}
	}
	for i, block := range s.Blocks {
		res.Blocks[i] = BlockSchema{
			Type:         block.GetType(),
			LabelNames:   block.GetLabels(),
			Unique:       block.GetUnique(),
			Required:     block.GetRequired(),
			UniqueLabels: block.GetUniqueLabels(),
			Body:         NewSchemaWithProto(block.GetBody()),
		}
	}
	return res
}

func (s *Schema) ToProto() *paproto.Schema {
	res := &paproto.Schema{
		Attributes: make([]*paproto.Schema_Attribute, len(s.Attributes)),
		Blocks:     make([]*paproto.Schema_Block, len(s.Blocks)),
	}
	for i, attr := range s.Attributes {
		res.Attributes[i] = &paproto.Schema_Attribute{
			Name:     attr.Name,
			Required: attr.Required,
		}
	}
	for i, block := range s.Blocks {
		res.Blocks[i] = &paproto.Schema_Block{
			Type:         block.Type,
			Labels:       block.LabelNames,
			Unique:       block.Unique,
			Required:     block.Required,
			UniqueLabels: block.UniqueLabels,
			Body:         block.Body.ToProto(),
		}
	}
	return res
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

func (m *GRPCServer) GetQuerySchema(ctx context.Context, _ *paproto.GetQuerySchema_Request) (*paproto.GetQuerySchema_Response, error) {
	schema, err := m.Impl.GetQuerySchema(ctx)
	if err != nil {
		return nil, err
	}
	res := &paproto.GetQuerySchema_Response{
		Schema: schema.ToProto(),
	}
	return res, nil
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

func (m *GRPCClient) GetQuerySchema(ctx context.Context) (*Schema, error) {
	r := &paproto.GetQuerySchema_Request{}
	res, err := m.client.GetQuerySchema(ctx, r)
	if err != nil {
		return nil, err
	}
	return NewSchemaWithProto(res.Schema), nil
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
