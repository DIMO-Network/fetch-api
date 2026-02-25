package graph

import (
	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/graph/model"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// filterToSearchOptions converts GraphQL filter and tokenID to grpc.SearchOptions.
func filterToSearchOptions(filter *model.CloudEventFilter, subject cloudevent.ERC721DID) *grpc.SearchOptions {
	opts := &grpc.SearchOptions{
		Subject: &wrapperspb.StringValue{Value: subject.String()},
	}
	if filter == nil {
		return opts
	}
	if filter.ID != nil {
		opts.Id = &wrapperspb.StringValue{Value: *filter.ID}
	}
	if filter.Type != nil {
		opts.Type = &wrapperspb.StringValue{Value: *filter.Type}
	}
	if filter.Dataversion != nil {
		opts.DataVersion = &wrapperspb.StringValue{Value: *filter.Dataversion}
	}
	if filter.Source != nil {
		opts.Source = &wrapperspb.StringValue{Value: *filter.Source}
	}
	if filter.Producer != nil {
		opts.Producer = &wrapperspb.StringValue{Value: *filter.Producer}
	}
	if filter.Before != nil {
		opts.Before = timestamppb.New(*filter.Before)
	}
	if filter.After != nil {
		opts.After = timestamppb.New(*filter.After)
	}
	return opts
}

const defaultLimit = 10

// Preallocated empty slices for list resolvers to avoid allocating on sql.ErrNoRows.
var (
	emptyCloudEventIndexList = []*model.CloudEventIndex{}
	emptyCloudEventList      = []*CloudEventWrapper{}
)

func resolveLimit(limit *int) int {
	if limit != nil && *limit > 0 {
		return *limit
	}
	return defaultLimit
}

// indexToModel converts a CloudEvent index entry to the GraphQL model,
// using the CloudEventHeader from the library directly.
func indexToModel(idx cloudevent.CloudEvent[eventrepo.ObjectInfo]) *model.CloudEventIndex {
	idx.CloudEventHeader.Tags = grpc.TagsOrEmpty(idx.CloudEventHeader.Tags)
	return &model.CloudEventIndex{
		Header:   &idx.CloudEventHeader,
		IndexKey: idx.Data.Key,
	}
}
