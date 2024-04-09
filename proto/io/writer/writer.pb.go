// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        (unknown)
// source: io/writer/writer.proto

package writer

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

type WriteRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// payload is the write request in bytes
	Payload []byte `protobuf:"bytes,1,opt,name=payload,proto3" json:"payload,omitempty"`
}

func (x *WriteRequest) Reset() {
	*x = WriteRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_io_writer_writer_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WriteRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WriteRequest) ProtoMessage() {}

func (x *WriteRequest) ProtoReflect() protoreflect.Message {
	mi := &file_io_writer_writer_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WriteRequest.ProtoReflect.Descriptor instead.
func (*WriteRequest) Descriptor() ([]byte, []int) {
	return file_io_writer_writer_proto_rawDescGZIP(), []int{0}
}

func (x *WriteRequest) GetPayload() []byte {
	if x != nil {
		return x.Payload
	}
	return nil
}

type WriteResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// written is the length of payload in bytes
	Written int32 `protobuf:"varint,1,opt,name=written,proto3" json:"written,omitempty"`
	// error is an error message
	Error *string `protobuf:"bytes,2,opt,name=error,proto3,oneof" json:"error,omitempty"`
}

func (x *WriteResponse) Reset() {
	*x = WriteResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_io_writer_writer_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WriteResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WriteResponse) ProtoMessage() {}

func (x *WriteResponse) ProtoReflect() protoreflect.Message {
	mi := &file_io_writer_writer_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WriteResponse.ProtoReflect.Descriptor instead.
func (*WriteResponse) Descriptor() ([]byte, []int) {
	return file_io_writer_writer_proto_rawDescGZIP(), []int{1}
}

func (x *WriteResponse) GetWritten() int32 {
	if x != nil {
		return x.Written
	}
	return 0
}

func (x *WriteResponse) GetError() string {
	if x != nil && x.Error != nil {
		return *x.Error
	}
	return ""
}

var File_io_writer_writer_proto protoreflect.FileDescriptor

var file_io_writer_writer_proto_rawDesc = []byte{
	0x0a, 0x16, 0x69, 0x6f, 0x2f, 0x77, 0x72, 0x69, 0x74, 0x65, 0x72, 0x2f, 0x77, 0x72, 0x69, 0x74,
	0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x09, 0x69, 0x6f, 0x2e, 0x77, 0x72, 0x69,
	0x74, 0x65, 0x72, 0x22, 0x28, 0x0a, 0x0c, 0x57, 0x72, 0x69, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x22, 0x4e, 0x0a,
	0x0d, 0x57, 0x72, 0x69, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x18,
	0x0a, 0x07, 0x77, 0x72, 0x69, 0x74, 0x74, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x07, 0x77, 0x72, 0x69, 0x74, 0x74, 0x65, 0x6e, 0x12, 0x19, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72,
	0x88, 0x01, 0x01, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x32, 0x44, 0x0a,
	0x06, 0x57, 0x72, 0x69, 0x74, 0x65, 0x72, 0x12, 0x3a, 0x0a, 0x05, 0x57, 0x72, 0x69, 0x74, 0x65,
	0x12, 0x17, 0x2e, 0x69, 0x6f, 0x2e, 0x77, 0x72, 0x69, 0x74, 0x65, 0x72, 0x2e, 0x57, 0x72, 0x69,
	0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x69, 0x6f, 0x2e, 0x77,
	0x72, 0x69, 0x74, 0x65, 0x72, 0x2e, 0x57, 0x72, 0x69, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x42, 0x37, 0x5a, 0x35, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x69, 0x64, 0x65, 0x72, 0x69, 0x74, 0x64, 0x6f, 0x6e, 0x65,
	0x2f, 0x6c, 0x61, 0x6e, 0x64, 0x73, 0x6c, 0x69, 0x64, 0x65, 0x76, 0x6d, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x69, 0x6f, 0x2f, 0x77, 0x72, 0x69, 0x74, 0x65, 0x72, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_io_writer_writer_proto_rawDescOnce sync.Once
	file_io_writer_writer_proto_rawDescData = file_io_writer_writer_proto_rawDesc
)

func file_io_writer_writer_proto_rawDescGZIP() []byte {
	file_io_writer_writer_proto_rawDescOnce.Do(func() {
		file_io_writer_writer_proto_rawDescData = protoimpl.X.CompressGZIP(file_io_writer_writer_proto_rawDescData)
	})
	return file_io_writer_writer_proto_rawDescData
}

var file_io_writer_writer_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_io_writer_writer_proto_goTypes = []interface{}{
	(*WriteRequest)(nil),  // 0: io.writer.WriteRequest
	(*WriteResponse)(nil), // 1: io.writer.WriteResponse
}
var file_io_writer_writer_proto_depIdxs = []int32{
	0, // 0: io.writer.Writer.Write:input_type -> io.writer.WriteRequest
	1, // 1: io.writer.Writer.Write:output_type -> io.writer.WriteResponse
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_io_writer_writer_proto_init() }
func file_io_writer_writer_proto_init() {
	if File_io_writer_writer_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_io_writer_writer_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WriteRequest); i {
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
		file_io_writer_writer_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WriteResponse); i {
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
	file_io_writer_writer_proto_msgTypes[1].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_io_writer_writer_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_io_writer_writer_proto_goTypes,
		DependencyIndexes: file_io_writer_writer_proto_depIdxs,
		MessageInfos:      file_io_writer_writer_proto_msgTypes,
	}.Build()
	File_io_writer_writer_proto = out.File
	file_io_writer_writer_proto_rawDesc = nil
	file_io_writer_writer_proto_goTypes = nil
	file_io_writer_writer_proto_depIdxs = nil
}
