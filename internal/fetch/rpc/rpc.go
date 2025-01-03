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
	"github.com/rs/zerolog/log"
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

// GetLatestIndex translates the gRPC call to the indexrepo type and returns the latest index for the given options.
func (s *Server) GetLatestIndex(ctx context.Context, req *grpc.GetLatestIndexRequest) (*grpc.GetLatestIndexResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	index, err := s.indexService.GetLatestIndex(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest index key: %w", err)
	}
	return &grpc.GetLatestIndexResponse{
		Index: &grpc.CloudEventIndex{
			Data: &grpc.ObjectInfo{
				Key: index.Data.Key,
			},
			Header: cloudEventHeaderToProto(&index.CloudEventHeader),
		},
	}, nil
}

// ListIndex translates the gRPC call to the indexrepo type and fetches index keys for the given options.
func (s *Server) ListIndex(ctx context.Context, req *grpc.ListIndexesRequest) (*grpc.ListIndexesResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	indexObjs, err := s.indexService.ListIndexes(ctx, int(req.GetLimit()), options)
	if err != nil {
		return nil, fmt.Errorf("failed to get index keys: %w", err)
	}
	indexList := make([]*grpc.CloudEventIndex, len(indexObjs))
	for i := range indexObjs {
		indexList[i] = &grpc.CloudEventIndex{
			Data: &grpc.ObjectInfo{
				Key: indexObjs[i].Data.Key,
			},
			Header: cloudEventHeaderToProto(&indexObjs[i].CloudEventHeader),
		}
	}
	return &grpc.ListIndexesResponse{Indexes: indexList}, nil
}

// ListCloudEvents translates the gRPC call to the indexrepo type and fetches data for the given options.
func (s *Server) ListCloudEvents(ctx context.Context, req *grpc.ListCloudEventsRequest) (*grpc.ListCloudEventsResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	metaList, err := s.indexService.ListIndexes(ctx, int(req.GetLimit()), options)
	if err != nil {
		log.Warn().Any("options", options).Any("req", req).Msg("failed to get objects")
		return nil, fmt.Errorf("failed to get objects: %w", err)
	}
	data, err := fetch.ListCloudEventsFromIndexes(ctx, s.indexService, metaList, []string{s.cloudEventBucket, s.ephemeralBucket})
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
	metdata, err := s.indexService.GetLatestIndex(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}
	latestData, err := fetch.GetCloudEventFromIndex(ctx, s.indexService, metdata, []string{s.cloudEventBucket, s.ephemeralBucket})
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}
	return &grpc.GetLatestCloudEventResponse{CloudEvent: cloudEventToProto(latestData)}, nil
}

// ListCloudEventsFromIndex translates the gRPC call to the indexrepo type and fetches data for the given index keys.
func (s *Server) ListCloudEventsFromIndex(ctx context.Context, req *grpc.ListCloudEventsFromKeysRequest) (*grpc.ListCloudEventsFromKeysResponse, error) {
	protoIndexList := req.GetIndexes()
	indexes := make([]cloudevent.CloudEvent[indexrepo.ObjectInfo], len(protoIndexList))
	for i, index := range protoIndexList {
		indexes[i] = cloudevent.CloudEvent[indexrepo.ObjectInfo]{
			CloudEventHeader: index.GetHeader().AsCloudEventHeader(),
			Data: indexrepo.ObjectInfo{
				Key: index.GetData().GetKey(),
			},
		}
	}
	data, err := fetch.ListCloudEventsFromIndexes(ctx, s.indexService, indexes, []string{s.cloudEventBucket, s.ephemeralBucket})
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
func translateSearchOptions(protoOptions *grpc.SearchOptions) *indexrepo.SearchOptions {
	if protoOptions == nil {
		return nil
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

	return &indexrepo.SearchOptions{
		After:        after,
		Before:       before,
		TimestampAsc: timestampAsc,
		Type:         getStringValue(protoOptions.GetType()),
		DataVersion:  getStringValue(protoOptions.GetDataVersion()),
		Subject:      getStringValue(protoOptions.GetSubject()),
		Source:       getStringValue(protoOptions.GetSource()),
		Producer:     getStringValue(protoOptions.GetProducer()),
		Extras:       getStringValue(protoOptions.GetExtras()),
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
