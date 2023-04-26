// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.12
// source: progress.proto

package progrock

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

type LogStream int32

const (
	LogStream_STDOUT LogStream = 0
	LogStream_STDERR LogStream = 1
)

// Enum value maps for LogStream.
var (
	LogStream_name = map[int32]string{
		0: "STDOUT",
		1: "STDERR",
	}
	LogStream_value = map[string]int32{
		"STDOUT": 0,
		"STDERR": 1,
	}
)

func (x LogStream) Enum() *LogStream {
	p := new(LogStream)
	*p = x
	return p
}

func (x LogStream) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LogStream) Descriptor() protoreflect.EnumDescriptor {
	return file_progress_proto_enumTypes[0].Descriptor()
}

func (LogStream) Type() protoreflect.EnumType {
	return &file_progress_proto_enumTypes[0]
}

func (x LogStream) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LogStream.Descriptor instead.
func (LogStream) EnumDescriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{0}
}

// StatusUpdate contains a snapshot of state updates for the graph.
type StatusUpdate struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Vertexes contains the set of updated vertexes.
	Vertexes []*Vertex `protobuf:"bytes,1,rep,name=vertexes,proto3" json:"vertexes,omitempty"`
	// Tasks contains the set of updated tasks.
	Tasks []*VertexTask `protobuf:"bytes,2,rep,name=tasks,proto3" json:"tasks,omitempty"`
	// Logs contains the set of new log output.
	Logs []*VertexLog `protobuf:"bytes,3,rep,name=logs,proto3" json:"logs,omitempty"`
}

func (x *StatusUpdate) Reset() {
	*x = StatusUpdate{}
	if protoimpl.UnsafeEnabled {
		mi := &file_progress_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StatusUpdate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StatusUpdate) ProtoMessage() {}

func (x *StatusUpdate) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StatusUpdate.ProtoReflect.Descriptor instead.
func (*StatusUpdate) Descriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{0}
}

func (x *StatusUpdate) GetVertexes() []*Vertex {
	if x != nil {
		return x.Vertexes
	}
	return nil
}

func (x *StatusUpdate) GetTasks() []*VertexTask {
	if x != nil {
		return x.Tasks
	}
	return nil
}

func (x *StatusUpdate) GetLogs() []*VertexLog {
	if x != nil {
		return x.Logs
	}
	return nil
}

// Group is used to group related vertexes.
type Group struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// ID is an arbitrary identifier for the group.
	Id          string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Parent      *string  `protobuf:"bytes,2,opt,name=parent,proto3,oneof" json:"parent,omitempty"`
	Name        string   `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	Description *string  `protobuf:"bytes,4,opt,name=description,proto3,oneof" json:"description,omitempty"`
	Labels      []*Label `protobuf:"bytes,5,rep,name=labels,proto3" json:"labels,omitempty"`
}

func (x *Group) Reset() {
	*x = Group{}
	if protoimpl.UnsafeEnabled {
		mi := &file_progress_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Group) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Group) ProtoMessage() {}

func (x *Group) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Group.ProtoReflect.Descriptor instead.
func (*Group) Descriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{1}
}

func (x *Group) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Group) GetParent() string {
	if x != nil && x.Parent != nil {
		return *x.Parent
	}
	return ""
}

func (x *Group) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Group) GetDescription() string {
	if x != nil && x.Description != nil {
		return *x.Description
	}
	return ""
}

func (x *Group) GetLabels() []*Label {
	if x != nil {
		return x.Labels
	}
	return nil
}

type Label struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name  string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Label) Reset() {
	*x = Label{}
	if protoimpl.UnsafeEnabled {
		mi := &file_progress_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Label) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Label) ProtoMessage() {}

func (x *Label) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Label.ProtoReflect.Descriptor instead.
func (*Label) Descriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{2}
}

func (x *Label) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Label) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

// Vertex is a node in the graph of work to be done.
type Vertex struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// ID is a unique identifier for the vertex, such as a digest.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Name is a user-visible name for the vertex.
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	// Inputs contains IDs of vertices that this vertex depends on.
	Inputs []string `protobuf:"bytes,3,rep,name=inputs,proto3" json:"inputs,omitempty"`
	// Outputs contains IDs of vertices that this vertex created.
	//
	// The intention is to allow a vertex to express that it created other
	// vertexes which may not maintain an input relationship to its creator, for
	// example an API request that created an object that may have otherwise been
	// created in some other way.
	Outputs []string `protobuf:"bytes,4,rep,name=outputs,proto3" json:"outputs,omitempty"`
	// Started is the time that the vertex started evaluating.
	Started *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=started,proto3,oneof" json:"started,omitempty"`
	// Completed is the time that the vertex finished evaluating.
	Completed *timestamppb.Timestamp `protobuf:"bytes,6,opt,name=completed,proto3,oneof" json:"completed,omitempty"`
	// Cached indicates whether the vertex resulted in a cache hit.
	Cached bool `protobuf:"varint,7,opt,name=cached,proto3" json:"cached,omitempty"`
	// Error is the error message, if any, that occurred while evaluating the
	// vertex.
	Error *string `protobuf:"bytes,8,opt,name=error,proto3,oneof" json:"error,omitempty"`
	// Canceled indicates whether the vertex was interrupted.
	Canceled bool `protobuf:"varint,9,opt,name=canceled,proto3" json:"canceled,omitempty"`
	// Internal indicates that the vertex should not visible to the user by
	// default, but may be revealed with a flag.
	Internal bool `protobuf:"varint,10,opt,name=internal,proto3" json:"internal,omitempty"`
	// Group associates the vertex to a group.
	Group *string `protobuf:"bytes,11,opt,name=group,proto3,oneof" json:"group,omitempty"`
}

func (x *Vertex) Reset() {
	*x = Vertex{}
	if protoimpl.UnsafeEnabled {
		mi := &file_progress_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Vertex) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Vertex) ProtoMessage() {}

func (x *Vertex) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Vertex.ProtoReflect.Descriptor instead.
func (*Vertex) Descriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{3}
}

func (x *Vertex) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Vertex) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Vertex) GetInputs() []string {
	if x != nil {
		return x.Inputs
	}
	return nil
}

func (x *Vertex) GetOutputs() []string {
	if x != nil {
		return x.Outputs
	}
	return nil
}

func (x *Vertex) GetStarted() *timestamppb.Timestamp {
	if x != nil {
		return x.Started
	}
	return nil
}

func (x *Vertex) GetCompleted() *timestamppb.Timestamp {
	if x != nil {
		return x.Completed
	}
	return nil
}

func (x *Vertex) GetCached() bool {
	if x != nil {
		return x.Cached
	}
	return false
}

func (x *Vertex) GetError() string {
	if x != nil && x.Error != nil {
		return *x.Error
	}
	return ""
}

func (x *Vertex) GetCanceled() bool {
	if x != nil {
		return x.Canceled
	}
	return false
}

func (x *Vertex) GetInternal() bool {
	if x != nil {
		return x.Internal
	}
	return false
}

func (x *Vertex) GetGroup() string {
	if x != nil && x.Group != nil {
		return *x.Group
	}
	return ""
}

// VertexTask is a task that a vertex is performing.
type VertexTask struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Vertex is the ID of the vertex that is performing the task.
	Vertex string `protobuf:"bytes,1,opt,name=vertex,proto3" json:"vertex,omitempty"`
	// Name is the user-visible name of the task.
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	// Total is the total number of units of work to be done, such as the number
	// of bytes to be downloaded.
	Total int64 `protobuf:"varint,3,opt,name=total,proto3" json:"total,omitempty"`
	// Current is the number of units of work that have been completed.
	Current int64 `protobuf:"varint,4,opt,name=current,proto3" json:"current,omitempty"`
	// Started is the time that the task started.
	Started *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=started,proto3,oneof" json:"started,omitempty"`
	// Completed is the time that the task finished.
	Completed *timestamppb.Timestamp `protobuf:"bytes,6,opt,name=completed,proto3,oneof" json:"completed,omitempty"`
}

func (x *VertexTask) Reset() {
	*x = VertexTask{}
	if protoimpl.UnsafeEnabled {
		mi := &file_progress_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VertexTask) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VertexTask) ProtoMessage() {}

func (x *VertexTask) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VertexTask.ProtoReflect.Descriptor instead.
func (*VertexTask) Descriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{4}
}

func (x *VertexTask) GetVertex() string {
	if x != nil {
		return x.Vertex
	}
	return ""
}

func (x *VertexTask) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *VertexTask) GetTotal() int64 {
	if x != nil {
		return x.Total
	}
	return 0
}

func (x *VertexTask) GetCurrent() int64 {
	if x != nil {
		return x.Current
	}
	return 0
}

func (x *VertexTask) GetStarted() *timestamppb.Timestamp {
	if x != nil {
		return x.Started
	}
	return nil
}

func (x *VertexTask) GetCompleted() *timestamppb.Timestamp {
	if x != nil {
		return x.Completed
	}
	return nil
}

// VertexLog is a log message from a vertex.
type VertexLog struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Vertex is the ID of the vertex that emitted the log message.
	Vertex string `protobuf:"bytes,1,opt,name=vertex,proto3" json:"vertex,omitempty"`
	// Stream is the stream that the log message was emitted to.
	Stream LogStream `protobuf:"varint,2,opt,name=stream,proto3,enum=progrock.LogStream" json:"stream,omitempty"`
	// Data is the chunk of log output.
	Data []byte `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
	// Timestamp is the time that the log message was emitted.
	Timestamp *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
}

func (x *VertexLog) Reset() {
	*x = VertexLog{}
	if protoimpl.UnsafeEnabled {
		mi := &file_progress_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VertexLog) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VertexLog) ProtoMessage() {}

func (x *VertexLog) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VertexLog.ProtoReflect.Descriptor instead.
func (*VertexLog) Descriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{5}
}

func (x *VertexLog) GetVertex() string {
	if x != nil {
		return x.Vertex
	}
	return ""
}

func (x *VertexLog) GetStream() LogStream {
	if x != nil {
		return x.Stream
	}
	return LogStream_STDOUT
}

func (x *VertexLog) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *VertexLog) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

var File_progress_proto protoreflect.FileDescriptor

var file_progress_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x08, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x6f, 0x63, 0x6b, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65,
	0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x91, 0x01, 0x0a, 0x0c,
	0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x2c, 0x0a, 0x08,
	0x76, 0x65, 0x72, 0x74, 0x65, 0x78, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x10,
	0x2e, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x6f, 0x63, 0x6b, 0x2e, 0x56, 0x65, 0x72, 0x74, 0x65, 0x78,
	0x52, 0x08, 0x76, 0x65, 0x72, 0x74, 0x65, 0x78, 0x65, 0x73, 0x12, 0x2a, 0x0a, 0x05, 0x74, 0x61,
	0x73, 0x6b, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x70, 0x72, 0x6f, 0x67,
	0x72, 0x6f, 0x63, 0x6b, 0x2e, 0x56, 0x65, 0x72, 0x74, 0x65, 0x78, 0x54, 0x61, 0x73, 0x6b, 0x52,
	0x05, 0x74, 0x61, 0x73, 0x6b, 0x73, 0x12, 0x27, 0x0a, 0x04, 0x6c, 0x6f, 0x67, 0x73, 0x18, 0x03,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x6f, 0x63, 0x6b, 0x2e,
	0x56, 0x65, 0x72, 0x74, 0x65, 0x78, 0x4c, 0x6f, 0x67, 0x52, 0x04, 0x6c, 0x6f, 0x67, 0x73, 0x22,
	0xb3, 0x01, 0x0a, 0x05, 0x47, 0x72, 0x6f, 0x75, 0x70, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1b, 0x0a, 0x06, 0x70, 0x61, 0x72,
	0x65, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x06, 0x70, 0x61, 0x72,
	0x65, 0x6e, 0x74, 0x88, 0x01, 0x01, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x25, 0x0a, 0x0b, 0x64, 0x65,
	0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48,
	0x01, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x88, 0x01,
	0x01, 0x12, 0x27, 0x0a, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x0f, 0x2e, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x6f, 0x63, 0x6b, 0x2e, 0x4c, 0x61, 0x62,
	0x65, 0x6c, 0x52, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x42, 0x09, 0x0a, 0x07, 0x5f, 0x70,
	0x61, 0x72, 0x65, 0x6e, 0x74, 0x42, 0x0e, 0x0a, 0x0c, 0x5f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x31, 0x0a, 0x05, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x8c, 0x03, 0x0a, 0x06, 0x56, 0x65, 0x72,
	0x74, 0x65, 0x78, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x69, 0x6e, 0x70, 0x75, 0x74,
	0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x69, 0x6e, 0x70, 0x75, 0x74, 0x73, 0x12,
	0x18, 0x0a, 0x07, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x07, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x73, 0x12, 0x39, 0x0a, 0x07, 0x73, 0x74, 0x61,
	0x72, 0x74, 0x65, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x48, 0x00, 0x52, 0x07, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65,
	0x64, 0x88, 0x01, 0x01, 0x12, 0x3d, 0x0a, 0x09, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x65,
	0x64, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x48, 0x01, 0x52, 0x09, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x65, 0x64,
	0x88, 0x01, 0x01, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x61, 0x63, 0x68, 0x65, 0x64, 0x18, 0x07, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x06, 0x63, 0x61, 0x63, 0x68, 0x65, 0x64, 0x12, 0x19, 0x0a, 0x05, 0x65,
	0x72, 0x72, 0x6f, 0x72, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x48, 0x02, 0x52, 0x05, 0x65, 0x72,
	0x72, 0x6f, 0x72, 0x88, 0x01, 0x01, 0x12, 0x1a, 0x0a, 0x08, 0x63, 0x61, 0x6e, 0x63, 0x65, 0x6c,
	0x65, 0x64, 0x18, 0x09, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x63, 0x61, 0x6e, 0x63, 0x65, 0x6c,
	0x65, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x18, 0x0a,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x12, 0x19,
	0x0a, 0x05, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x09, 0x48, 0x03, 0x52,
	0x05, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x88, 0x01, 0x01, 0x42, 0x0a, 0x0a, 0x08, 0x5f, 0x73, 0x74,
	0x61, 0x72, 0x74, 0x65, 0x64, 0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65,
	0x74, 0x65, 0x64, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x42, 0x08, 0x0a,
	0x06, 0x5f, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x22, 0xfc, 0x01, 0x0a, 0x0a, 0x56, 0x65, 0x72, 0x74,
	0x65, 0x78, 0x54, 0x61, 0x73, 0x6b, 0x12, 0x16, 0x0a, 0x06, 0x76, 0x65, 0x72, 0x74, 0x65, 0x78,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x76, 0x65, 0x72, 0x74, 0x65, 0x78, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x75, 0x72, 0x72,
	0x65, 0x6e, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x63, 0x75, 0x72, 0x72, 0x65,
	0x6e, 0x74, 0x12, 0x39, 0x0a, 0x07, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65, 0x64, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x48,
	0x00, 0x52, 0x07, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65, 0x64, 0x88, 0x01, 0x01, 0x12, 0x3d, 0x0a,
	0x09, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x48, 0x01, 0x52, 0x09,
	0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x88, 0x01, 0x01, 0x42, 0x0a, 0x0a, 0x08,
	0x5f, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65, 0x64, 0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x63, 0x6f, 0x6d,
	0x70, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x22, 0x9e, 0x01, 0x0a, 0x09, 0x56, 0x65, 0x72, 0x74, 0x65,
	0x78, 0x4c, 0x6f, 0x67, 0x12, 0x16, 0x0a, 0x06, 0x76, 0x65, 0x72, 0x74, 0x65, 0x78, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x76, 0x65, 0x72, 0x74, 0x65, 0x78, 0x12, 0x2b, 0x0a, 0x06,
	0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x13, 0x2e, 0x70,
	0x72, 0x6f, 0x67, 0x72, 0x6f, 0x63, 0x6b, 0x2e, 0x4c, 0x6f, 0x67, 0x53, 0x74, 0x72, 0x65, 0x61,
	0x6d, 0x52, 0x06, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x38, 0x0a,
	0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2a, 0x23, 0x0a, 0x09, 0x4c, 0x6f, 0x67, 0x53, 0x74,
	0x72, 0x65, 0x61, 0x6d, 0x12, 0x0a, 0x0a, 0x06, 0x53, 0x54, 0x44, 0x4f, 0x55, 0x54, 0x10, 0x00,
	0x12, 0x0a, 0x0a, 0x06, 0x53, 0x54, 0x44, 0x45, 0x52, 0x52, 0x10, 0x01, 0x42, 0x1a, 0x5a, 0x18,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x76, 0x69, 0x74, 0x6f, 0x2f,
	0x70, 0x72, 0x6f, 0x67, 0x72, 0x6f, 0x63, 0x6b, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_progress_proto_rawDescOnce sync.Once
	file_progress_proto_rawDescData = file_progress_proto_rawDesc
)

func file_progress_proto_rawDescGZIP() []byte {
	file_progress_proto_rawDescOnce.Do(func() {
		file_progress_proto_rawDescData = protoimpl.X.CompressGZIP(file_progress_proto_rawDescData)
	})
	return file_progress_proto_rawDescData
}

var file_progress_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_progress_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_progress_proto_goTypes = []interface{}{
	(LogStream)(0),                // 0: progrock.LogStream
	(*StatusUpdate)(nil),          // 1: progrock.StatusUpdate
	(*Group)(nil),                 // 2: progrock.Group
	(*Label)(nil),                 // 3: progrock.Label
	(*Vertex)(nil),                // 4: progrock.Vertex
	(*VertexTask)(nil),            // 5: progrock.VertexTask
	(*VertexLog)(nil),             // 6: progrock.VertexLog
	(*timestamppb.Timestamp)(nil), // 7: google.protobuf.Timestamp
}
var file_progress_proto_depIdxs = []int32{
	4,  // 0: progrock.StatusUpdate.vertexes:type_name -> progrock.Vertex
	5,  // 1: progrock.StatusUpdate.tasks:type_name -> progrock.VertexTask
	6,  // 2: progrock.StatusUpdate.logs:type_name -> progrock.VertexLog
	3,  // 3: progrock.Group.labels:type_name -> progrock.Label
	7,  // 4: progrock.Vertex.started:type_name -> google.protobuf.Timestamp
	7,  // 5: progrock.Vertex.completed:type_name -> google.protobuf.Timestamp
	7,  // 6: progrock.VertexTask.started:type_name -> google.protobuf.Timestamp
	7,  // 7: progrock.VertexTask.completed:type_name -> google.protobuf.Timestamp
	0,  // 8: progrock.VertexLog.stream:type_name -> progrock.LogStream
	7,  // 9: progrock.VertexLog.timestamp:type_name -> google.protobuf.Timestamp
	10, // [10:10] is the sub-list for method output_type
	10, // [10:10] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_progress_proto_init() }
func file_progress_proto_init() {
	if File_progress_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_progress_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StatusUpdate); i {
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
		file_progress_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Group); i {
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
		file_progress_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Label); i {
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
		file_progress_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Vertex); i {
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
		file_progress_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VertexTask); i {
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
		file_progress_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VertexLog); i {
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
	file_progress_proto_msgTypes[1].OneofWrappers = []interface{}{}
	file_progress_proto_msgTypes[3].OneofWrappers = []interface{}{}
	file_progress_proto_msgTypes[4].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_progress_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_progress_proto_goTypes,
		DependencyIndexes: file_progress_proto_depIdxs,
		EnumInfos:         file_progress_proto_enumTypes,
		MessageInfos:      file_progress_proto_msgTypes,
	}.Build()
	File_progress_proto = out.File
	file_progress_proto_rawDesc = nil
	file_progress_proto_goTypes = nil
	file_progress_proto_depIdxs = nil
}
