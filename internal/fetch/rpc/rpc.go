// Package rpc provides the gRPC server implementation for the index repo service.
package rpc

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/fetch"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server is used to implement grpc.IndexRepoServiceServer.
type Server struct {
	eventService *eventrepo.Service
	grpc.UnimplementedFetchServiceServer
	buckets []string
}

// NewServer creates a new Server instance.
func NewServer(chConn clickhouse.Conn, objGetter eventrepo.ObjectGetter, buckets []string) *Server {
	return &Server{
		eventService: eventrepo.New(chConn, objGetter),
		buckets:      buckets,
	}
}

// GetLatestIndex translates the gRPC call to the indexrepo type and returns the latest index for the given options.
func (s *Server) GetLatestIndex(ctx context.Context, req *grpc.GetLatestIndexRequest) (*grpc.GetLatestIndexResponse, error) {
	var index cloudevent.CloudEvent[eventrepo.ObjectInfo]
	var err error

	if req.GetAdvancedOptions() != nil {
		index, err = s.eventService.GetLatestIndexAdvanced(ctx, req.GetAdvancedOptions())
	} else {
		index, err = s.eventService.GetLatestIndex(ctx, req.GetOptions())
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "no index key Found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get latest index: %v", err)
	}
	return &grpc.GetLatestIndexResponse{
		Index: &grpc.CloudEventIndex{
			Data: &grpc.ObjectInfo{
				Key: index.Data.Key,
			},
			Header: grpc.CloudEventHeaderToProto(&index.CloudEventHeader),
		},
	}, nil
}

// ListIndexes translates the gRPC call to the indexrepo type and fetches index keys for the given options.
func (s *Server) ListIndexes(ctx context.Context, req *grpc.ListIndexesRequest) (*grpc.ListIndexesResponse, error) {
	var indexObjs []cloudevent.CloudEvent[eventrepo.ObjectInfo]
	var err error

	if req.GetAdvancedOptions() != nil {
		indexObjs, err = s.eventService.ListIndexesAdvanced(ctx, int(req.GetLimit()), req.GetAdvancedOptions())
	} else {
		indexObjs, err = s.eventService.ListIndexes(ctx, int(req.GetLimit()), req.GetOptions())
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "no index key Found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get index keys: %v", err)
	}
	indexList := make([]*grpc.CloudEventIndex, len(indexObjs))
	for i := range indexObjs {
		indexList[i] = &grpc.CloudEventIndex{
			Data: &grpc.ObjectInfo{
				Key: indexObjs[i].Data.Key,
			},
			Header: grpc.CloudEventHeaderToProto(&indexObjs[i].CloudEventHeader),
		}
	}
	return &grpc.ListIndexesResponse{Indexes: indexList}, nil
}

// ListCloudEvents translates the gRPC call to the indexrepo type and fetches data for the given options.
func (s *Server) ListCloudEvents(ctx context.Context, req *grpc.ListCloudEventsRequest) (*grpc.ListCloudEventsResponse, error) {
	var metaList []cloudevent.CloudEvent[eventrepo.ObjectInfo]
	var err error

	if req.GetAdvancedOptions() != nil {
		metaList, err = s.eventService.ListIndexesAdvanced(ctx, int(req.GetLimit()), req.GetAdvancedOptions())
	} else {
		metaList, err = s.eventService.ListIndexes(ctx, int(req.GetLimit()), req.GetOptions())
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "no index keys Found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get index keys: %v", err)
	}
	data, err := fetch.ListCloudEventsFromIndexes(ctx, s.eventService, metaList, s.buckets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get objects: %v", err)
	}

	events := make([]*grpc.CloudEvent, len(data))
	for i, d := range data {
		events[i] = grpc.CloudEventToProto(d)
	}
	return &grpc.ListCloudEventsResponse{CloudEvents: events}, nil
}

// GetLatestCloudEvent translates the gRPC call to the indexrepo type and fetches the latest data for the given options.
func (s *Server) GetLatestCloudEvent(ctx context.Context, req *grpc.GetLatestCloudEventRequest) (*grpc.GetLatestCloudEventResponse, error) {
	var metdata cloudevent.CloudEvent[eventrepo.ObjectInfo]
	var err error

	if req.GetAdvancedOptions() != nil {
		metdata, err = s.eventService.GetLatestIndexAdvanced(ctx, req.GetAdvancedOptions())
	} else {
		metdata, err = s.eventService.GetLatestIndex(ctx, req.GetOptions())
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "no index key Found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get latest index: %v", err)
	}
	latestData, err := fetch.GetCloudEventFromIndex(ctx, s.eventService, metdata, s.buckets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest object: %v", err)
	}
	return &grpc.GetLatestCloudEventResponse{CloudEvent: grpc.CloudEventToProto(latestData)}, nil
}

// ListCloudEventsFromIndex translates the gRPC call to the indexrepo type and fetches data for the given index keys.
func (s *Server) ListCloudEventsFromIndex(ctx context.Context, req *grpc.ListCloudEventsFromKeysRequest) (*grpc.ListCloudEventsFromKeysResponse, error) {
	protoIndexList := req.GetIndexes()
	events := make([]cloudevent.CloudEvent[eventrepo.ObjectInfo], len(protoIndexList))
	for i, index := range protoIndexList {
		events[i] = cloudevent.CloudEvent[eventrepo.ObjectInfo]{
			CloudEventHeader: index.GetHeader().AsCloudEventHeader(),
			Data: eventrepo.ObjectInfo{
				Key: index.GetData().GetKey(),
			},
		}
	}
	data, err := fetch.ListCloudEventsFromIndexes(ctx, s.eventService, events, s.buckets)
	if err != nil {
		notFoundErr := &types.NoSuchKey{}
		if errors.As(err, &notFoundErr) {
			return nil, status.Errorf(codes.NotFound, "no objects found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get objects: %v", err)
	}
	dataObjects := make([]*grpc.CloudEvent, len(data))
	for i, d := range data {
		dataObjects[i] = grpc.CloudEventToProto(d)
	}
	return &grpc.ListCloudEventsFromKeysResponse{CloudEvents: dataObjects}, nil
}
