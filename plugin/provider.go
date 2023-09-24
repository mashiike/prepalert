package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

type RunQueryRequest struct {
	ProviderParameters *provider.ProviderParameter
	QueryName          string
	QueryParameters    json.RawMessage
}

type RunQueryResponse struct {
	Name      string
	Query     string
	Params    []json.RawMessage
	Columns   []string
	Rows      [][]string
	JSONLines []json.RawMessage
}

type Provider interface {
	ValidateProviderParameter(ctx context.Context, pp *provider.ProviderParameter) error
	GetQuerySchema(ctx context.Context) (*Schema, error)
	RunQuery(ctx context.Context, req *RunQueryRequest) (*RunQueryResponse, error)
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
	pp := &provider.ProviderParameter{
		Type:   params.GetType(),
		Name:   params.GetName(),
		Params: json.RawMessage(params.GetJson()),
	}
	err := m.Impl.ValidateProviderParameter(ctx, pp)
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

func (m *GRPCServer) RunQuery(ctx context.Context, r *paproto.RunQuery_Request) (*paproto.RunQuery_Response, error) {
	pp := r.GetProviderParams()
	qp := r.GetQueryParams()
	res, err := m.Impl.RunQuery(
		ctx,
		&RunQueryRequest{
			ProviderParameters: &provider.ProviderParameter{
				Type:   pp.GetType(),
				Name:   pp.GetName(),
				Params: json.RawMessage(pp.GetJson()),
			},
			QueryName:       r.QueryName,
			QueryParameters: json.RawMessage(qp),
		},
	)
	if err != nil {
		return nil, err
	}
	params := make([]string, len(res.Params))
	for i, param := range res.Params {
		params[i] = string(param)
	}
	if res.Columns != nil {
		rows := make([]*paproto.RunQuery_Response_Row, len(res.Rows))
		for i, row := range res.Rows {
			if len(row) > len(res.Columns) {
				return nil, fmt.Errorf("invalid row[%d] length: %d > %d", i, len(row), len(res.Columns))
			}
			r := &paproto.RunQuery_Response_Row{
				Values: make([]string, len(res.Columns)),
			}
			for j, val := range row {
				r.Values[j] = val
			}
			rows[i] = r
		}
		return &paproto.RunQuery_Response{
			Name:    res.Name,
			Query:   res.Query,
			Params:  params,
			Columns: res.Columns,
			Rows:    rows,
		}, nil
	}
	jsonLines := make([]string, len(res.JSONLines))
	for i, line := range res.JSONLines {
		jsonLines[i] = string(line)
	}
	return &paproto.RunQuery_Response{
		Name:      res.Name,
		Query:     res.Query,
		Params:    params,
		Jsonlines: jsonLines,
	}, nil
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

func (m *GRPCClient) RunQuery(ctx context.Context, req *RunQueryRequest) (*RunQueryResponse, error) {
	r := &paproto.RunQuery_Request{
		ProviderParams: &paproto.ProviderParameter{
			Type: req.ProviderParameters.Type,
			Name: req.ProviderParameters.Name,
			Json: string(req.ProviderParameters.Params),
		},

		QueryParams: string(req.QueryParameters),
	}
	res, err := m.client.RunQuery(ctx, r)
	if err != nil {
		return nil, err
	}
	rows := make([][]string, len(res.Rows))
	for i, row := range res.Rows {
		rows[i] = row.Values
	}
	jsonLines := make([]json.RawMessage, len(res.Jsonlines))
	for i, line := range res.Jsonlines {
		jsonLines[i] = json.RawMessage(line)
	}
	params := make([]json.RawMessage, len(res.Params))
	for i, param := range res.Params {
		params[i] = json.RawMessage(param)
	}
	return &RunQueryResponse{
		Name:      res.Name,
		Query:     res.Query,
		Params:    params,
		Columns:   res.Columns,
		Rows:      rows,
		JSONLines: jsonLines,
	}, nil
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
