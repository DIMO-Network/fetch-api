// Package rpc provides the gRPC server implementation for the index repo service.
package rpc

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/nameindexer/pkg/clickhouse/indexrepo"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Server is used to implement grpc.IndexRepoServiceServer.
type Server struct {
	indexService *indexrepo.Service
	grpc.UnimplementedFetchServiceServer
	cloudEventBucket string
	ephemeralBucket  string
}

// New creates a new Server instance.
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
	data, err := s.indexService.GetObject(ctx, s.cloudEventBucket, int(req.GetLimit()), options)
	if err != nil {
		return nil, fmt.Errorf("failed to get objects: %w", err)
	}
	dataObjects := make([]*grpc.DataObject, len(data))
	for i, d := range data {
		dataObjects[i] = &grpc.DataObject{
			IndexKey: d.IndexKey,
			Data:     d.Data,
		}
	}
	return &grpc.GetObjectsResponse{DataObjects: dataObjects}, nil
}

// GetLatestObject translates the gRPC call to the indexrepo type and fetches the latest data for the given options.
func (s *Server) GetLatestObject(ctx context.Context, req *grpc.GetLatestObjectRequest) (*grpc.GetLatestObjectResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	latestData, err := s.indexService.GetLatestObject(ctx, s.cloudEventBucket, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest object: %w", err)
	}
	return &grpc.GetLatestObjectResponse{DataObject: &grpc.DataObject{
		IndexKey: latestData.IndexKey,
		Data:     latestData.Data,
	}}, nil
}

// GetObjectsFromIndexKeys translates the gRPC call to the indexrepo type and fetches data for the given options.
func (s *Server) GetObjectsFromIndexKeys(ctx context.Context, req *grpc.GetObjectsFromIndexKeysRequest) (*grpc.GetObjectsFromIndexKeysResponse, error) {
	data, err := s.indexService.GetObjectsFromIndexKeys(ctx, req.GetIndexKeys(), s.cloudEventBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get objects: %w", err)
	}
	dataObjects := make([]*grpc.DataObject, len(data))
	for i, d := range data {
		dataObjects[i] = &grpc.DataObject{
			IndexKey: d.IndexKey,
			Data:     d.Data,
		}
	}
	return &grpc.GetObjectsFromIndexKeysResponse{DataObjects: dataObjects}, nil
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

	return indexrepo.SearchOptions{
		After:           after,
		Before:          before,
		TimestampAsc:    timestampAsc,
		PrimaryFiller:   getStringValue(protoOptions.GetPrimaryFiller()),
		DataType:        getStringValue(protoOptions.GetDataType()),
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
