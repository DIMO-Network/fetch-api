// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.1
// 	protoc        v5.28.3
// source: pkg/grpc/cloudevent.proto

package grpc

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// CloudEvent represents a CloudEvent with a header and data.
type CloudEvent struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Header        *CloudEventHeader      `protobuf:"bytes,1,opt,name=header,proto3" json:"header,omitempty"`
	Data          []byte                 `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CloudEvent) Reset() {
	*x = CloudEvent{}
	mi := &file_pkg_grpc_cloudevent_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CloudEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloudEvent) ProtoMessage() {}

func (x *CloudEvent) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_grpc_cloudevent_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloudEvent.ProtoReflect.Descriptor instead.
func (*CloudEvent) Descriptor() ([]byte, []int) {
	return file_pkg_grpc_cloudevent_proto_rawDescGZIP(), []int{0}
}

func (x *CloudEvent) GetHeader() *CloudEventHeader {
	if x != nil {
		return x.Header
	}
	return nil
}

func (x *CloudEvent) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

// CloudEventHeader represents the header structure of a CloudEvent.
type CloudEventHeader struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// ID is an identifier for the event. The combination of ID and Source must
	// be unique.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Source is the context in which the event happened. In a distributed system it might consist of multiple Producers.
	// Typically the 0x address.
	Source string `protobuf:"bytes,2,opt,name=source,proto3" json:"source,omitempty"`
	// Producer is a specific instance, process or device that creates the data structure describing the CloudEvent.
	// Typically a DID of a nft.
	Producer string `protobuf:"bytes,3,opt,name=producer,proto3" json:"producer,omitempty"`
	// SpecVersion is the version of CloudEvents specification used.
	// This is always hardcoded "1.0".
	SpecVersion string `protobuf:"bytes,4,opt,name=spec_version,json=specVersion,proto3" json:"spec_version,omitempty"`
	// Subject is an optional field identifying the subject of the event within
	// the context of the event producer.
	// Typically the DID of the nft.
	Subject string `protobuf:"bytes,5,opt,name=subject,proto3" json:"subject,omitempty"`
	// Time which the event occurred.
	Time *timestamppb.Timestamp `protobuf:"bytes,6,opt,name=time,proto3" json:"time,omitempty"`
	// Type describes the type of event.
	// Typically a one of the predefined DIMO types. (dimo.status, dimo.fingerprint, dimo.verfiabaleCredential...)
	Type string `protobuf:"bytes,7,opt,name=type,proto3" json:"type,omitempty"`
	// DataContentType is an optional MIME type for the data field. We almost
	// always serialize to JSON and in that case this field is implicitly
	// "application/json".
	DataContentType string `protobuf:"bytes,8,opt,name=data_content_type,json=dataContentType,proto3" json:"data_content_type,omitempty"`
	// DataSchema is an optional URI pointing to a schema for the data field.
	DataSchema string `protobuf:"bytes,9,opt,name=data_schema,json=dataSchema,proto3" json:"data_schema,omitempty"`
	// DataVersion is the controlled by the source of the event and is used to provide information about the data.
	DataVersion string `protobuf:"bytes,10,opt,name=data_version,json=dataVersion,proto3" json:"data_version,omitempty"`
	// Extras contains any additional fields that are not part of the CloudEvent excluding the data field.
	Extras        map[string][]byte `protobuf:"bytes,11,rep,name=extras,proto3" json:"extras,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CloudEventHeader) Reset() {
	*x = CloudEventHeader{}
	mi := &file_pkg_grpc_cloudevent_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CloudEventHeader) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloudEventHeader) ProtoMessage() {}

func (x *CloudEventHeader) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_grpc_cloudevent_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloudEventHeader.ProtoReflect.Descriptor instead.
func (*CloudEventHeader) Descriptor() ([]byte, []int) {
	return file_pkg_grpc_cloudevent_proto_rawDescGZIP(), []int{1}
}

func (x *CloudEventHeader) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *CloudEventHeader) GetSource() string {
	if x != nil {
		return x.Source
	}
	return ""
}

func (x *CloudEventHeader) GetProducer() string {
	if x != nil {
		return x.Producer
	}
	return ""
}

func (x *CloudEventHeader) GetSpecVersion() string {
	if x != nil {
		return x.SpecVersion
	}
	return ""
}

func (x *CloudEventHeader) GetSubject() string {
	if x != nil {
		return x.Subject
	}
	return ""
}

func (x *CloudEventHeader) GetTime() *timestamppb.Timestamp {
	if x != nil {
		return x.Time
	}
	return nil
}

func (x *CloudEventHeader) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *CloudEventHeader) GetDataContentType() string {
	if x != nil {
		return x.DataContentType
	}
	return ""
}

func (x *CloudEventHeader) GetDataSchema() string {
	if x != nil {
		return x.DataSchema
	}
	return ""
}

func (x *CloudEventHeader) GetDataVersion() string {
	if x != nil {
		return x.DataVersion
	}
	return ""
}

func (x *CloudEventHeader) GetExtras() map[string][]byte {
	if x != nil {
		return x.Extras
	}
	return nil
}

var File_pkg_grpc_cloudevent_proto protoreflect.FileDescriptor

var file_pkg_grpc_cloudevent_proto_rawDesc = []byte{
	0x0a, 0x19, 0x70, 0x6b, 0x67, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x2f, 0x63, 0x6c, 0x6f, 0x75, 0x64,
	0x65, 0x76, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0a, 0x63, 0x6c, 0x6f,
	0x75, 0x64, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x56, 0x0a, 0x0a, 0x43, 0x6c, 0x6f, 0x75,
	0x64, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x12, 0x34, 0x0a, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x65, 0x76,
	0x65, 0x6e, 0x74, 0x2e, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x48, 0x65,
	0x61, 0x64, 0x65, 0x72, 0x52, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04,
	0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61,
	0x22, 0xc4, 0x03, 0x0a, 0x10, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x48,
	0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x12, 0x1a, 0x0a,
	0x08, 0x70, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x70, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x65, 0x72, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x70, 0x65,
	0x63, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0b, 0x73, 0x70, 0x65, 0x63, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07,
	0x73, 0x75, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x73,
	0x75, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x12, 0x2e, 0x0a, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x52, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x07,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x2a, 0x0a, 0x11, 0x64, 0x61,
	0x74, 0x61, 0x5f, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x64, 0x61, 0x74, 0x61, 0x43, 0x6f, 0x6e, 0x74, 0x65,
	0x6e, 0x74, 0x54, 0x79, 0x70, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x64, 0x61, 0x74, 0x61, 0x5f, 0x73,
	0x63, 0x68, 0x65, 0x6d, 0x61, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x64, 0x61, 0x74,
	0x61, 0x53, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x12, 0x21, 0x0a, 0x0c, 0x64, 0x61, 0x74, 0x61, 0x5f,
	0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64,
	0x61, 0x74, 0x61, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x40, 0x0a, 0x06, 0x65, 0x78,
	0x74, 0x72, 0x61, 0x73, 0x18, 0x0b, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x28, 0x2e, 0x63, 0x6c, 0x6f,
	0x75, 0x64, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x2e, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x45, 0x76, 0x65,
	0x6e, 0x74, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x2e, 0x45, 0x78, 0x74, 0x72, 0x61, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x06, 0x65, 0x78, 0x74, 0x72, 0x61, 0x73, 0x1a, 0x39, 0x0a, 0x0b,
	0x45, 0x78, 0x74, 0x72, 0x61, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x42, 0x2c, 0x5a, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x44, 0x49, 0x4d, 0x4f, 0x2d, 0x4e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x2f, 0x66, 0x65, 0x74, 0x63, 0x68, 0x2d, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x6b, 0x67,
	0x2f, 0x67, 0x72, 0x70, 0x63, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_grpc_cloudevent_proto_rawDescOnce sync.Once
	file_pkg_grpc_cloudevent_proto_rawDescData = file_pkg_grpc_cloudevent_proto_rawDesc
)

func file_pkg_grpc_cloudevent_proto_rawDescGZIP() []byte {
	file_pkg_grpc_cloudevent_proto_rawDescOnce.Do(func() {
		file_pkg_grpc_cloudevent_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_grpc_cloudevent_proto_rawDescData)
	})
	return file_pkg_grpc_cloudevent_proto_rawDescData
}

var file_pkg_grpc_cloudevent_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_pkg_grpc_cloudevent_proto_goTypes = []any{
	(*CloudEvent)(nil),            // 0: cloudevent.CloudEvent
	(*CloudEventHeader)(nil),      // 1: cloudevent.CloudEventHeader
	nil,                           // 2: cloudevent.CloudEventHeader.ExtrasEntry
	(*timestamppb.Timestamp)(nil), // 3: google.protobuf.Timestamp
}
var file_pkg_grpc_cloudevent_proto_depIdxs = []int32{
	1, // 0: cloudevent.CloudEvent.header:type_name -> cloudevent.CloudEventHeader
	3, // 1: cloudevent.CloudEventHeader.time:type_name -> google.protobuf.Timestamp
	2, // 2: cloudevent.CloudEventHeader.extras:type_name -> cloudevent.CloudEventHeader.ExtrasEntry
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_pkg_grpc_cloudevent_proto_init() }
func file_pkg_grpc_cloudevent_proto_init() {
	if File_pkg_grpc_cloudevent_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_grpc_cloudevent_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_grpc_cloudevent_proto_goTypes,
		DependencyIndexes: file_pkg_grpc_cloudevent_proto_depIdxs,
		MessageInfos:      file_pkg_grpc_cloudevent_proto_msgTypes,
	}.Build()
	File_pkg_grpc_cloudevent_proto = out.File
	file_pkg_grpc_cloudevent_proto_rawDesc = nil
	file_pkg_grpc_cloudevent_proto_goTypes = nil
	file_pkg_grpc_cloudevent_proto_depIdxs = nil
}
