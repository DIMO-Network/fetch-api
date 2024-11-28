// Package rpc provides the gRPC server implementation for the index repo service.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DIMO-Network/fetch-api/internal/fetch"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/model-garage/pkg/cloudevent"
	"github.com/DIMO-Network/nameindexer/pkg/clickhouse/indexrepo"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Server is used to implement grpc.IndexRepoServiceServer.
type Server struct {
	indexService *indexrepo.Service
	grpc.UnimplementedFetchServiceServer
	cloudEventBucket string
	ephemeralBucket  string
}

// NewServer creates a new Server instance.
func NewServer(chConn clickhouse.Conn, objGetter indexrepo.ObjectGetter, cloudEventBucket, ephemeralBucket string) *Server {
	return &Server{
		indexService:     indexrepo.New(chConn, objGetter),
		cloudEventBucket: cloudEventBucket,
		ephemeralBucket:  ephemeralBucket,
	}
}

// GetLatestMetadata translates the gRPC call to the indexrepo type and returns the latest metadata for the given options.
func (s *Server) GetLatestMetadata(ctx context.Context, req *grpc.GetLatestMetadataRequest) (*grpc.GetLatestMetadataResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	metadata, err := s.indexService.GetLatestMetadataFromRaw(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest index key: %w", err)
	}
	return &grpc.GetLatestMetadataResponse{
		Metadata: &grpc.CloudEventMetadata{
			Key:    metadata.Key,
			Header: cloudEventHeaderToProto(&metadata.CloudEventHeader),
		},
	}, nil
}

// ListMetadata translates the gRPC call to the indexrepo type and fetches index keys for the given options.
func (s *Server) ListMetadata(ctx context.Context, req *grpc.ListMetadataRequest) (*grpc.ListMetadataResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	metaDataObjs, err := s.indexService.ListMetadataFromRaw(ctx, int(req.GetLimit()), options)
	if err != nil {
		return nil, fmt.Errorf("failed to get index keys: %w", err)
	}
	metadataList := make([]*grpc.CloudEventMetadata, len(metaDataObjs))
	for i := range metaDataObjs {
		metadataList[i] = &grpc.CloudEventMetadata{
			Key:    metaDataObjs[i].Key,
			Header: cloudEventHeaderToProto(&metaDataObjs[i].CloudEventHeader),
		}
	}
	return &grpc.ListMetadataResponse{MetadataList: metadataList}, nil
}

// ListCloudEvents translates the gRPC call to the indexrepo type and fetches data for the given options.
func (s *Server) ListCloudEvents(ctx context.Context, req *grpc.ListCloudEventsRequest) (*grpc.ListCloudEventsResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	metaList, err := s.indexService.ListMetadataFromRaw(ctx, int(req.GetLimit()), options)
	if err != nil {
		return nil, fmt.Errorf("failed to get objects: %w", err)
	}
	data, err := fetch.ListCloudEventsFromMetadata(ctx, s.indexService, metaList, []string{s.cloudEventBucket, s.ephemeralBucket})
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}

	events := make([]*grpc.CloudEvent, len(data))
	for i, d := range data {
		events[i] = cloudEventToProto(d)
	}
	return &grpc.ListCloudEventsResponse{CloudEvents: events}, nil
}

// GetLatestCloudEvent translates the gRPC call to the indexrepo type and fetches the latest data for the given options.
func (s *Server) GetLatestCloudEvent(ctx context.Context, req *grpc.GetLatestCloudEventRequest) (*grpc.GetLatestCloudEventResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	metdata, err := s.indexService.GetLatestMetadataFromRaw(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}
	latestData, err := fetch.GetCloudEventFromKey(ctx, s.indexService, metdata.Key, []string{s.cloudEventBucket, s.ephemeralBucket})
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}
	return &grpc.GetLatestCloudEventResponse{CloudEvent: cloudEventToProto(latestData)}, nil
}

// ListCloudEventsFromKeys translates the gRPC call to the indexrepo type and fetches data for the given options.
func (s *Server) ListCloudEventsFromKeys(ctx context.Context, req *grpc.ListCloudEventsFromKeysRequest) (*grpc.ListCloudEventsFromKeysResponse, error) {
	data, err := fetch.GetCloudEventsFromKeys(ctx, s.indexService, req.GetIndexKeys(), []string{s.cloudEventBucket, s.ephemeralBucket})
	if err != nil {
		return nil, fmt.Errorf("failed to get objects: %w", err)
	}
	dataObjects := make([]*grpc.CloudEvent, len(data))
	for i, d := range data {
		dataObjects[i] = cloudEventToProto(d)
	}
	return &grpc.ListCloudEventsFromKeysResponse{CloudEvents: dataObjects}, nil
}

// translateProtoToSearchOptions translates a SearchOptions proto message to the Go SearchOptions type.
func translateSearchOptions(protoOptions *grpc.SearchOptions) indexrepo.RawSearchOptions {
	if protoOptions == nil {
		return indexrepo.RawSearchOptions{}
	}

	// Converting after timestamp from proto to Go time
	var after time.Time
	if protoOptions.GetAfter() != nil {
		after = protoOptions.GetAfter().AsTime()
	}

	// Converting before timestamp from proto to Go time
	var before time.Time
	if protoOptions.GetBefore() != nil {
		before = protoOptions.GetBefore().AsTime()
	}

	// Converting boolean value for timestamp ascending
	var timestampAsc bool
	if protoOptions.GetTimestampAsc() != nil {
		timestampAsc = protoOptions.GetTimestampAsc().GetValue()
	}

	return indexrepo.RawSearchOptions{
		After:        after,
		Before:       before,
		TimestampAsc: timestampAsc,
		Type:         getStringValue(protoOptions.GetType()),
		DataVersion:  getStringValue(protoOptions.GetDataVersion()),
		Subject:      getStringValue(protoOptions.GetSubject()),
		Source:       getStringValue(protoOptions.GetSource()),
		Producer:     getStringValue(protoOptions.GetProducer()),
		Optional:     getStringValue(protoOptions.GetOptional()),
	}
}

// getStringValue extracts the value from a wrappersgrpc.StringValue, returning an empty string if nil.
func getStringValue(protoStr *wrapperspb.StringValue) *string {
	if protoStr == nil {
		return nil
	}
	val := protoStr.GetValue()
	return &val
}

func cloudEventHeaderToProto(event *cloudevent.CloudEventHeader) *grpc.CloudEventHeader {
	if event == nil {
		return nil
	}
	extras := make(map[string][]byte)
	for k, v := range event.Extras {
		v, err := json.Marshal(v)
		if err != nil {
			// Skip the extra if it can't be marshaled
			continue
		}
		extras[k] = v
	}
	return &grpc.CloudEventHeader{
		Id:              event.ID,
		Source:          event.Source,
		Producer:        event.Producer,
		Subject:         event.Subject,
		SpecVersion:     event.SpecVersion,
		Time:            timestamppb.New(event.Time),
		Type:            event.Type,
		DataContentType: event.DataContentType,
		DataSchema:      event.DataSchema,
		DataVersion:     event.DataVersion,
		Extras:          extras,
	}
}

func cloudEventToProto(event cloudevent.CloudEvent[json.RawMessage]) *grpc.CloudEvent {
	extras := make(map[string][]byte)
	for k, v := range event.Extras {
		v, err := json.Marshal(v)
		if err != nil {
			// Skip the extra if it can't be marshaled
			continue
		}
		extras[k] = v
	}
	return &grpc.CloudEvent{
		Header: cloudEventHeaderToProto(&event.CloudEventHeader),
		Data:   event.Data,
	}
}
