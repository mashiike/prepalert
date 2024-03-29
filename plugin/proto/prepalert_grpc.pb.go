// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v4.24.3
// source: plugin/proto/prepalert.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ProviderClient is the client API for Provider service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ProviderClient interface {
	ValidateProviderParameter(ctx context.Context, in *ValidatProviderPaameter_Request, opts ...grpc.CallOption) (*ValidatProviderPaameter_Response, error)
	GetQuerySchema(ctx context.Context, in *GetQuerySchema_Request, opts ...grpc.CallOption) (*GetQuerySchema_Response, error)
	RunQuery(ctx context.Context, in *RunQuery_Request, opts ...grpc.CallOption) (*RunQuery_Response, error)
}

type providerClient struct {
	cc grpc.ClientConnInterface
}

func NewProviderClient(cc grpc.ClientConnInterface) ProviderClient {
	return &providerClient{cc}
}

func (c *providerClient) ValidateProviderParameter(ctx context.Context, in *ValidatProviderPaameter_Request, opts ...grpc.CallOption) (*ValidatProviderPaameter_Response, error) {
	out := new(ValidatProviderPaameter_Response)
	err := c.cc.Invoke(ctx, "/prepalert.Provider/ValidateProviderParameter", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *providerClient) GetQuerySchema(ctx context.Context, in *GetQuerySchema_Request, opts ...grpc.CallOption) (*GetQuerySchema_Response, error) {
	out := new(GetQuerySchema_Response)
	err := c.cc.Invoke(ctx, "/prepalert.Provider/GetQuerySchema", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *providerClient) RunQuery(ctx context.Context, in *RunQuery_Request, opts ...grpc.CallOption) (*RunQuery_Response, error) {
	out := new(RunQuery_Response)
	err := c.cc.Invoke(ctx, "/prepalert.Provider/RunQuery", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ProviderServer is the server API for Provider service.
// All implementations must embed UnimplementedProviderServer
// for forward compatibility
type ProviderServer interface {
	ValidateProviderParameter(context.Context, *ValidatProviderPaameter_Request) (*ValidatProviderPaameter_Response, error)
	GetQuerySchema(context.Context, *GetQuerySchema_Request) (*GetQuerySchema_Response, error)
	RunQuery(context.Context, *RunQuery_Request) (*RunQuery_Response, error)
	mustEmbedUnimplementedProviderServer()
}

// UnimplementedProviderServer must be embedded to have forward compatible implementations.
type UnimplementedProviderServer struct {
}

func (UnimplementedProviderServer) ValidateProviderParameter(context.Context, *ValidatProviderPaameter_Request) (*ValidatProviderPaameter_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateProviderParameter not implemented")
}
func (UnimplementedProviderServer) GetQuerySchema(context.Context, *GetQuerySchema_Request) (*GetQuerySchema_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetQuerySchema not implemented")
}
func (UnimplementedProviderServer) RunQuery(context.Context, *RunQuery_Request) (*RunQuery_Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunQuery not implemented")
}
func (UnimplementedProviderServer) mustEmbedUnimplementedProviderServer() {}

// UnsafeProviderServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ProviderServer will
// result in compilation errors.
type UnsafeProviderServer interface {
	mustEmbedUnimplementedProviderServer()
}

func RegisterProviderServer(s grpc.ServiceRegistrar, srv ProviderServer) {
	s.RegisterService(&Provider_ServiceDesc, srv)
}

func _Provider_ValidateProviderParameter_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidatProviderPaameter_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServer).ValidateProviderParameter(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/prepalert.Provider/ValidateProviderParameter",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServer).ValidateProviderParameter(ctx, req.(*ValidatProviderPaameter_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _Provider_GetQuerySchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetQuerySchema_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServer).GetQuerySchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/prepalert.Provider/GetQuerySchema",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServer).GetQuerySchema(ctx, req.(*GetQuerySchema_Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _Provider_RunQuery_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunQuery_Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServer).RunQuery(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/prepalert.Provider/RunQuery",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServer).RunQuery(ctx, req.(*RunQuery_Request))
	}
	return interceptor(ctx, in, info, handler)
}

// Provider_ServiceDesc is the grpc.ServiceDesc for Provider service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Provider_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "prepalert.Provider",
	HandlerType: (*ProviderServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ValidateProviderParameter",
			Handler:    _Provider_ValidateProviderParameter_Handler,
		},
		{
			MethodName: "GetQuerySchema",
			Handler:    _Provider_GetQuerySchema_Handler,
		},
		{
			MethodName: "RunQuery",
			Handler:    _Provider_RunQuery_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "plugin/proto/prepalert.proto",
}
