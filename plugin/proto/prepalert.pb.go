// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: plugin/proto/prepalert.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ProviderParameter struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type string `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Json string `protobuf:"bytes,3,opt,name=json,proto3" json:"json,omitempty"`
}

func (x *ProviderParameter) Reset() {
	*x = ProviderParameter{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ProviderParameter) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProviderParameter) ProtoMessage() {}

func (x *ProviderParameter) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProviderParameter.ProtoReflect.Descriptor instead.
func (*ProviderParameter) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{0}
}

func (x *ProviderParameter) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *ProviderParameter) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ProviderParameter) GetJson() string {
	if x != nil {
		return x.Json
	}
	return ""
}

type Schema struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Attributes []*Schema_Attribute `protobuf:"bytes,1,rep,name=attributes,proto3" json:"attributes,omitempty"`
	Blocks     []*Schema_Block     `protobuf:"bytes,2,rep,name=blocks,proto3" json:"blocks,omitempty"`
}

func (x *Schema) Reset() {
	*x = Schema{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Schema) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Schema) ProtoMessage() {}

func (x *Schema) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Schema.ProtoReflect.Descriptor instead.
func (*Schema) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{1}
}

func (x *Schema) GetAttributes() []*Schema_Attribute {
	if x != nil {
		return x.Attributes
	}
	return nil
}

func (x *Schema) GetBlocks() []*Schema_Block {
	if x != nil {
		return x.Blocks
	}
	return nil
}

type ValidatProviderPaameter struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ValidatProviderPaameter) Reset() {
	*x = ValidatProviderPaameter{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ValidatProviderPaameter) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ValidatProviderPaameter) ProtoMessage() {}

func (x *ValidatProviderPaameter) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ValidatProviderPaameter.ProtoReflect.Descriptor instead.
func (*ValidatProviderPaameter) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{2}
}

type GetQuerySchema struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetQuerySchema) Reset() {
	*x = GetQuerySchema{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetQuerySchema) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetQuerySchema) ProtoMessage() {}

func (x *GetQuerySchema) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetQuerySchema.ProtoReflect.Descriptor instead.
func (*GetQuerySchema) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{3}
}

type RunQuery struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *RunQuery) Reset() {
	*x = RunQuery{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunQuery) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunQuery) ProtoMessage() {}

func (x *RunQuery) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunQuery.ProtoReflect.Descriptor instead.
func (*RunQuery) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{4}
}

type Schema_Attribute struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name     string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Required bool   `protobuf:"varint,2,opt,name=required,proto3" json:"required,omitempty"`
}

func (x *Schema_Attribute) Reset() {
	*x = Schema_Attribute{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Schema_Attribute) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Schema_Attribute) ProtoMessage() {}

func (x *Schema_Attribute) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Schema_Attribute.ProtoReflect.Descriptor instead.
func (*Schema_Attribute) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{1, 0}
}

func (x *Schema_Attribute) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Schema_Attribute) GetRequired() bool {
	if x != nil {
		return x.Required
	}
	return false
}

type Schema_Block struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type         string   `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`
	Labels       []string `protobuf:"bytes,2,rep,name=labels,proto3" json:"labels,omitempty"`
	Unique       bool     `protobuf:"varint,3,opt,name=unique,proto3" json:"unique,omitempty"`
	Required     bool     `protobuf:"varint,4,opt,name=required,proto3" json:"required,omitempty"`
	UniqueLabels bool     `protobuf:"varint,5,opt,name=uniqueLabels,proto3" json:"uniqueLabels,omitempty"`
	Body         *Schema  `protobuf:"bytes,6,opt,name=body,proto3" json:"body,omitempty"`
}

func (x *Schema_Block) Reset() {
	*x = Schema_Block{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Schema_Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Schema_Block) ProtoMessage() {}

func (x *Schema_Block) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Schema_Block.ProtoReflect.Descriptor instead.
func (*Schema_Block) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{1, 1}
}

func (x *Schema_Block) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Schema_Block) GetLabels() []string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *Schema_Block) GetUnique() bool {
	if x != nil {
		return x.Unique
	}
	return false
}

func (x *Schema_Block) GetRequired() bool {
	if x != nil {
		return x.Required
	}
	return false
}

func (x *Schema_Block) GetUniqueLabels() bool {
	if x != nil {
		return x.UniqueLabels
	}
	return false
}

func (x *Schema_Block) GetBody() *Schema {
	if x != nil {
		return x.Body
	}
	return nil
}

type ValidatProviderPaameter_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Parameter *ProviderParameter `protobuf:"bytes,1,opt,name=parameter,proto3" json:"parameter,omitempty"`
}

func (x *ValidatProviderPaameter_Request) Reset() {
	*x = ValidatProviderPaameter_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ValidatProviderPaameter_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ValidatProviderPaameter_Request) ProtoMessage() {}

func (x *ValidatProviderPaameter_Request) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ValidatProviderPaameter_Request.ProtoReflect.Descriptor instead.
func (*ValidatProviderPaameter_Request) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{2, 0}
}

func (x *ValidatProviderPaameter_Request) GetParameter() *ProviderParameter {
	if x != nil {
		return x.Parameter
	}
	return nil
}

type ValidatProviderPaameter_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ok      bool   `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *ValidatProviderPaameter_Response) Reset() {
	*x = ValidatProviderPaameter_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ValidatProviderPaameter_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ValidatProviderPaameter_Response) ProtoMessage() {}

func (x *ValidatProviderPaameter_Response) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ValidatProviderPaameter_Response.ProtoReflect.Descriptor instead.
func (*ValidatProviderPaameter_Response) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{2, 1}
}

func (x *ValidatProviderPaameter_Response) GetOk() bool {
	if x != nil {
		return x.Ok
	}
	return false
}

func (x *ValidatProviderPaameter_Response) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type GetQuerySchema_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetQuerySchema_Request) Reset() {
	*x = GetQuerySchema_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetQuerySchema_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetQuerySchema_Request) ProtoMessage() {}

func (x *GetQuerySchema_Request) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetQuerySchema_Request.ProtoReflect.Descriptor instead.
func (*GetQuerySchema_Request) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{3, 0}
}

type GetQuerySchema_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Schema *Schema `protobuf:"bytes,1,opt,name=schema,proto3" json:"schema,omitempty"`
}

func (x *GetQuerySchema_Response) Reset() {
	*x = GetQuerySchema_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetQuerySchema_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetQuerySchema_Response) ProtoMessage() {}

func (x *GetQuerySchema_Response) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetQuerySchema_Response.ProtoReflect.Descriptor instead.
func (*GetQuerySchema_Response) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{3, 1}
}

func (x *GetQuerySchema_Response) GetSchema() *Schema {
	if x != nil {
		return x.Schema
	}
	return nil
}

type RunQuery_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ProviderParams *ProviderParameter `protobuf:"bytes,1,opt,name=providerParams,proto3" json:"providerParams,omitempty"`
	QueryParams    string             `protobuf:"bytes,2,opt,name=queryParams,proto3" json:"queryParams,omitempty"`
}

func (x *RunQuery_Request) Reset() {
	*x = RunQuery_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunQuery_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunQuery_Request) ProtoMessage() {}

func (x *RunQuery_Request) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunQuery_Request.ProtoReflect.Descriptor instead.
func (*RunQuery_Request) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{4, 0}
}

func (x *RunQuery_Request) GetProviderParams() *ProviderParameter {
	if x != nil {
		return x.ProviderParams
	}
	return nil
}

func (x *RunQuery_Request) GetQueryParams() string {
	if x != nil {
		return x.QueryParams
	}
	return ""
}

type RunQuery_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name      string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Query     string   `protobuf:"bytes,2,opt,name=query,proto3" json:"query,omitempty"`
	Params    []string `protobuf:"bytes,3,rep,name=params,proto3" json:"params,omitempty"`
	Jsonlines []string `protobuf:"bytes,4,rep,name=jsonlines,proto3" json:"jsonlines,omitempty"`
}

func (x *RunQuery_Response) Reset() {
	*x = RunQuery_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_plugin_proto_prepalert_proto_msgTypes[12]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunQuery_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunQuery_Response) ProtoMessage() {}

func (x *RunQuery_Response) ProtoReflect() protoreflect.Message {
	mi := &file_plugin_proto_prepalert_proto_msgTypes[12]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunQuery_Response.ProtoReflect.Descriptor instead.
func (*RunQuery_Response) Descriptor() ([]byte, []int) {
	return file_plugin_proto_prepalert_proto_rawDescGZIP(), []int{4, 1}
}

func (x *RunQuery_Response) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *RunQuery_Response) GetQuery() string {
	if x != nil {
		return x.Query
	}
	return ""
}

func (x *RunQuery_Response) GetParams() []string {
	if x != nil {
		return x.Params
	}
	return nil
}

func (x *RunQuery_Response) GetJsonlines() []string {
	if x != nil {
		return x.Jsonlines
	}
	return nil
}

var File_plugin_proto_prepalert_proto protoreflect.FileDescriptor

var file_plugin_proto_prepalert_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70,
	0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x09,
	0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x22, 0x4f, 0x0a, 0x11, 0x50, 0x72, 0x6f,
	0x76, 0x69, 0x64, 0x65, 0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x12, 0x12,
	0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6a, 0x73, 0x6f, 0x6e, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6a, 0x73, 0x6f, 0x6e, 0x22, 0xe8, 0x02, 0x0a, 0x06, 0x53,
	0x63, 0x68, 0x65, 0x6d, 0x61, 0x12, 0x3b, 0x0a, 0x0a, 0x61, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75,
	0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x70, 0x72, 0x65, 0x70,
	0x61, 0x6c, 0x65, 0x72, 0x74, 0x2e, 0x53, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x41, 0x74, 0x74,
	0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x52, 0x0a, 0x61, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74,
	0x65, 0x73, 0x12, 0x2f, 0x0a, 0x06, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x17, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x2e, 0x53,
	0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x06, 0x62, 0x6c, 0x6f,
	0x63, 0x6b, 0x73, 0x1a, 0x3b, 0x0a, 0x09, 0x41, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74, 0x65,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64,
	0x1a, 0xb2, 0x01, 0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06,
	0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x75, 0x6e, 0x69, 0x71, 0x75, 0x65,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06, 0x75, 0x6e, 0x69, 0x71, 0x75, 0x65, 0x12, 0x1a,
	0x0a, 0x08, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x08, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x12, 0x22, 0x0a, 0x0c, 0x75, 0x6e,
	0x69, 0x71, 0x75, 0x65, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x0c, 0x75, 0x6e, 0x69, 0x71, 0x75, 0x65, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x12, 0x25,
	0x0a, 0x04, 0x62, 0x6f, 0x64, 0x79, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x70,
	0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x2e, 0x53, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x52,
	0x04, 0x62, 0x6f, 0x64, 0x79, 0x22, 0x96, 0x01, 0x0a, 0x17, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61,
	0x74, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x50, 0x61, 0x61, 0x6d, 0x65, 0x74, 0x65,
	0x72, 0x1a, 0x45, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3a, 0x0a, 0x09,
	0x70, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1c, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x2e, 0x50, 0x72, 0x6f, 0x76,
	0x69, 0x64, 0x65, 0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x52, 0x09, 0x70,
	0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x1a, 0x34, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x02, 0x6f, 0x6b, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x52,
	0x0a, 0x0e, 0x47, 0x65, 0x74, 0x51, 0x75, 0x65, 0x72, 0x79, 0x53, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x1a, 0x09, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x35, 0x0a, 0x08, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x29, 0x0a, 0x06, 0x73, 0x63, 0x68, 0x65, 0x6d,
	0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c,
	0x65, 0x72, 0x74, 0x2e, 0x53, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x52, 0x06, 0x73, 0x63, 0x68, 0x65,
	0x6d, 0x61, 0x22, 0xe9, 0x01, 0x0a, 0x08, 0x52, 0x75, 0x6e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x1a,
	0x71, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x44, 0x0a, 0x0e, 0x70, 0x72,
	0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x2e, 0x50,
	0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72,
	0x52, 0x0e, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73,
	0x12, 0x20, 0x0a, 0x0b, 0x71, 0x75, 0x65, 0x72, 0x79, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x71, 0x75, 0x65, 0x72, 0x79, 0x50, 0x61, 0x72, 0x61,
	0x6d, 0x73, 0x1a, 0x6a, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x71, 0x75, 0x65, 0x72, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x71, 0x75, 0x65, 0x72, 0x79, 0x12, 0x16, 0x0a, 0x06, 0x70, 0x61, 0x72, 0x61,
	0x6d, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x73,
	0x12, 0x1c, 0x0a, 0x09, 0x6a, 0x73, 0x6f, 0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x73, 0x18, 0x04, 0x20,
	0x03, 0x28, 0x09, 0x52, 0x09, 0x6a, 0x73, 0x6f, 0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x73, 0x32, 0xa0,
	0x02, 0x0a, 0x08, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x12, 0x74, 0x0a, 0x19, 0x56,
	0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x50,
	0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x12, 0x2a, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61,
	0x6c, 0x65, 0x72, 0x74, 0x2e, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x50, 0x72, 0x6f, 0x76,
	0x69, 0x64, 0x65, 0x72, 0x50, 0x61, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x2e, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x2b, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74,
	0x2e, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72,
	0x50, 0x61, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x57, 0x0a, 0x0e, 0x47, 0x65, 0x74, 0x51, 0x75, 0x65, 0x72, 0x79, 0x53, 0x63, 0x68,
	0x65, 0x6d, 0x61, 0x12, 0x21, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x2e,
	0x47, 0x65, 0x74, 0x51, 0x75, 0x65, 0x72, 0x79, 0x53, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x22, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65,
	0x72, 0x74, 0x2e, 0x47, 0x65, 0x74, 0x51, 0x75, 0x65, 0x72, 0x79, 0x53, 0x63, 0x68, 0x65, 0x6d,
	0x61, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x45, 0x0a, 0x08, 0x52, 0x75,
	0x6e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x12, 0x1b, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65,
	0x72, 0x74, 0x2e, 0x52, 0x75, 0x6e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x2e, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x1c, 0x2e, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x2e,
	0x52, 0x75, 0x6e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x42, 0x2c, 0x5a, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x6d, 0x61, 0x73, 0x68, 0x69, 0x69, 0x6b, 0x65, 0x2f, 0x70, 0x72, 0x65, 0x70, 0x61, 0x6c, 0x65,
	0x72, 0x74, 0x2f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_plugin_proto_prepalert_proto_rawDescOnce sync.Once
	file_plugin_proto_prepalert_proto_rawDescData = file_plugin_proto_prepalert_proto_rawDesc
)

func file_plugin_proto_prepalert_proto_rawDescGZIP() []byte {
	file_plugin_proto_prepalert_proto_rawDescOnce.Do(func() {
		file_plugin_proto_prepalert_proto_rawDescData = protoimpl.X.CompressGZIP(file_plugin_proto_prepalert_proto_rawDescData)
	})
	return file_plugin_proto_prepalert_proto_rawDescData
}

var file_plugin_proto_prepalert_proto_msgTypes = make([]protoimpl.MessageInfo, 13)
var file_plugin_proto_prepalert_proto_goTypes = []interface{}{
	(*ProviderParameter)(nil),                // 0: prepalert.ProviderParameter
	(*Schema)(nil),                           // 1: prepalert.Schema
	(*ValidatProviderPaameter)(nil),          // 2: prepalert.ValidatProviderPaameter
	(*GetQuerySchema)(nil),                   // 3: prepalert.GetQuerySchema
	(*RunQuery)(nil),                         // 4: prepalert.RunQuery
	(*Schema_Attribute)(nil),                 // 5: prepalert.Schema.Attribute
	(*Schema_Block)(nil),                     // 6: prepalert.Schema.Block
	(*ValidatProviderPaameter_Request)(nil),  // 7: prepalert.ValidatProviderPaameter.Request
	(*ValidatProviderPaameter_Response)(nil), // 8: prepalert.ValidatProviderPaameter.Response
	(*GetQuerySchema_Request)(nil),           // 9: prepalert.GetQuerySchema.Request
	(*GetQuerySchema_Response)(nil),          // 10: prepalert.GetQuerySchema.Response
	(*RunQuery_Request)(nil),                 // 11: prepalert.RunQuery.Request
	(*RunQuery_Response)(nil),                // 12: prepalert.RunQuery.Response
}
var file_plugin_proto_prepalert_proto_depIdxs = []int32{
	5,  // 0: prepalert.Schema.attributes:type_name -> prepalert.Schema.Attribute
	6,  // 1: prepalert.Schema.blocks:type_name -> prepalert.Schema.Block
	1,  // 2: prepalert.Schema.Block.body:type_name -> prepalert.Schema
	0,  // 3: prepalert.ValidatProviderPaameter.Request.parameter:type_name -> prepalert.ProviderParameter
	1,  // 4: prepalert.GetQuerySchema.Response.schema:type_name -> prepalert.Schema
	0,  // 5: prepalert.RunQuery.Request.providerParams:type_name -> prepalert.ProviderParameter
	7,  // 6: prepalert.Provider.ValidateProviderParameter:input_type -> prepalert.ValidatProviderPaameter.Request
	9,  // 7: prepalert.Provider.GetQuerySchema:input_type -> prepalert.GetQuerySchema.Request
	11, // 8: prepalert.Provider.RunQuery:input_type -> prepalert.RunQuery.Request
	8,  // 9: prepalert.Provider.ValidateProviderParameter:output_type -> prepalert.ValidatProviderPaameter.Response
	10, // 10: prepalert.Provider.GetQuerySchema:output_type -> prepalert.GetQuerySchema.Response
	12, // 11: prepalert.Provider.RunQuery:output_type -> prepalert.RunQuery.Response
	9,  // [9:12] is the sub-list for method output_type
	6,  // [6:9] is the sub-list for method input_type
	6,  // [6:6] is the sub-list for extension type_name
	6,  // [6:6] is the sub-list for extension extendee
	0,  // [0:6] is the sub-list for field type_name
}

func init() { file_plugin_proto_prepalert_proto_init() }
func file_plugin_proto_prepalert_proto_init() {
	if File_plugin_proto_prepalert_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_plugin_proto_prepalert_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ProviderParameter); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Schema); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ValidatProviderPaameter); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetQuerySchema); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RunQuery); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Schema_Attribute); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Schema_Block); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ValidatProviderPaameter_Request); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ValidatProviderPaameter_Response); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetQuerySchema_Request); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetQuerySchema_Response); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RunQuery_Request); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_plugin_proto_prepalert_proto_msgTypes[12].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RunQuery_Response); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_plugin_proto_prepalert_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   13,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_plugin_proto_prepalert_proto_goTypes,
		DependencyIndexes: file_plugin_proto_prepalert_proto_depIdxs,
		MessageInfos:      file_plugin_proto_prepalert_proto_msgTypes,
	}.Build()
	File_plugin_proto_prepalert_proto = out.File
	file_plugin_proto_prepalert_proto_rawDesc = nil
	file_plugin_proto_prepalert_proto_goTypes = nil
	file_plugin_proto_prepalert_proto_depIdxs = nil
}
