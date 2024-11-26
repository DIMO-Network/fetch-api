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
	"github.com/DIMO-Network/nameindexer"
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

// GetLatestIndexKey translates the gRPC call to the indexrepo type and returns the latest index key for the given options.
func (s *Server) GetLatestIndexKey(ctx context.Context, req *grpc.GetLatestIndexKeyRequest) (*grpc.GetLatestIndexKeyResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	indexKey, err := s.indexService.GetLatestIndexKey(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest index key: %w", err)
	}
	return &grpc.GetLatestIndexKeyResponse{IndexKey: indexKey}, nil
}

// GetIndexKeys translates the gRPC call to the indexrepo type and fetches index keys for the given options.
func (s *Server) GetIndexKeys(ctx context.Context, req *grpc.GetIndexKeysRequest) (*grpc.GetIndexKeysResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	indexKeys, err := s.indexService.GetIndexKeys(ctx, int(req.GetLimit()), options)
	if err != nil {
		return nil, fmt.Errorf("failed to get index keys: %w", err)
	}
	return &grpc.GetIndexKeysResponse{IndexKeys: indexKeys}, nil
}

// GetObjects translates the gRPC call to the indexrepo type and fetches data for the given options.
func (s *Server) GetObjects(ctx context.Context, req *grpc.GetObjectsRequest) (*grpc.GetObjectsResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	idxKeys, err := s.indexService.GetIndexKeys(ctx, int(req.GetLimit()), options)
	if err != nil {
		return nil, fmt.Errorf("failed to get objects: %w", err)
	}
	data, err := fetch.GetObjectsFromIndexs(ctx, s.indexService, idxKeys, []string{s.cloudEventBucket, s.ephemeralBucket})
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}

	events := make([]*grpc.CloudEvent, len(data))
	for i, d := range data {
		events[i] = cloudEventToProto(d)
	}
	return &grpc.GetObjectsResponse{CloudEvents: events}, nil
}

// GetLatestObject translates the gRPC call to the indexrepo type and fetches the latest data for the given options.
func (s *Server) GetLatestObject(ctx context.Context, req *grpc.GetLatestObjectRequest) (*grpc.GetLatestObjectResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	idxKey, err := s.indexService.GetLatestIndexKey(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}
	latestData, err := fetch.GetObjectFromIndex(ctx, s.indexService, idxKey, []string{s.cloudEventBucket, s.ephemeralBucket})
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}
	return &grpc.GetLatestObjectResponse{CloudEvent: cloudEventToProto(latestData)}, nil
}

// GetObjectsFromIndexKeys translates the gRPC call to the indexrepo type and fetches data for the given options.
func (s *Server) GetObjectsFromIndexKeys(ctx context.Context, req *grpc.GetObjectsFromIndexKeysRequest) (*grpc.GetObjectsFromIndexKeysResponse, error) {
	data, err := fetch.GetObjectsFromIndexs(ctx, s.indexService, req.GetIndexKeys(), []string{s.cloudEventBucket, s.ephemeralBucket})
	if err != nil {
		return nil, fmt.Errorf("failed to get objects: %w", err)
	}
	dataObjects := make([]*grpc.CloudEvent, len(data))
	for i, d := range data {
		dataObjects[i] = cloudEventToProto(d)
	}
	return &grpc.GetObjectsFromIndexKeysResponse{CloudEvents: dataObjects}, nil
}

// translateProtoToSearchOptions translates a SearchOptions proto message to the Go SearchOptions type.
func translateSearchOptions(protoOptions *grpc.SearchOptions) indexrepo.SearchOptions {
	if protoOptions == nil {
		return indexrepo.SearchOptions{}
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
	var filler *string
	if protoOptions.GetType() != nil {
		val := protoOptions.GetType().GetValue()
		val = nameindexer.CloudTypeToFiller(val)
		filler = &val
	}
	return indexrepo.SearchOptions{
		After:           after,
		Before:          before,
		TimestampAsc:    timestampAsc,
		PrimaryFiller:   filler,
		DataType:        getStringValue(protoOptions.GetDataVersion()),
		Subject:         getStringValue(protoOptions.GetSubject()),
		SecondaryFiller: getStringValue(protoOptions.GetSecondaryFiller()),
		Source:          getStringValue(protoOptions.GetSource()),
		Producer:        getStringValue(protoOptions.GetProducer()),
		Optional:        getStringValue(protoOptions.GetOptional()),
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
		Header: &grpc.CloudEventHeader{
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
		},
		Data: event.Data,
	}
}
