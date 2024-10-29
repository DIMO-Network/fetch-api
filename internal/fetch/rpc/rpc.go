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
	grpc.UnimplementedIndexRepoServiceServer
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

// GetLatestFileName translates the gRPC call to the indexrepo type and returns the latest filename for the given options.
func (s *Server) GetLatestFileName(ctx context.Context, req *grpc.GetLatestFileNameRequest) (*grpc.GetLatestFileNameResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	filename, err := s.indexService.GetLatestFileName(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest file name: %w", err)
	}
	return &grpc.GetLatestFileNameResponse{Filename: filename}, nil
}

// GetFileNames translates the gRPC call to the indexrepo type and fetches filenames for the given options.
func (s *Server) GetFileNames(ctx context.Context, req *grpc.GetFileNamesRequest) (*grpc.GetFileNamesResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	filenames, err := s.indexService.GetFileNames(ctx, int(req.GetLimit()), options)
	if err != nil {
		return nil, fmt.Errorf("failed to get file names: %w", err)
	}
	return &grpc.GetFileNamesResponse{Filenames: filenames}, nil
}

// GetFiles translates the gRPC call to the indexrepo type and fetches data for the given options.
func (s *Server) GetFiles(ctx context.Context, req *grpc.GetFilesRequest) (*grpc.GetFilesResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	data, err := s.indexService.GetData(ctx, s.cloudEventBucket, int(req.GetLimit()), options)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}
	return &grpc.GetFilesResponse{Data: data}, nil
}

// GetLatestFile translates the gRPC call to the indexrepo type and fetches the latest data for the given options.
func (s *Server) GetLatestFile(ctx context.Context, req *grpc.GetLatestFileRequest) (*grpc.GetLatestFileResponse, error) {
	options := translateSearchOptions(req.GetOptions())
	latestData, err := s.indexService.GetLatestData(ctx, s.cloudEventBucket, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest file: %w", err)
	}
	return &grpc.GetLatestFileResponse{Data: latestData}, nil
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
