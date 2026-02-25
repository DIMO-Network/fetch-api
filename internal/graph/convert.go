package graph

import (
	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/graph/model"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// filterToAdvancedSearchOptions converts a GraphQL filter and subject DID to grpc.AdvancedSearchOptions.
func filterToAdvancedSearchOptions(filter *model.CloudEventFilter, subject cloudevent.ERC721DID) *grpc.AdvancedSearchOptions {
	opts := &grpc.AdvancedSearchOptions{
		Subject: &grpc.StringFilterOption{In: []string{subject.String()}},
	}
	if filter == nil {
		return opts
	}
	if filter.ID != nil {
		opts.Id = &grpc.StringFilterOption{In: []string{*filter.ID}}
	}
	if filter.Type != nil {
		opts.Type = &grpc.StringFilterOption{In: []string{*filter.Type}}
	}
	// dataversion (exact) and dataversions (list) are merged into a single In filter.
	var dvIn []string
	if filter.Dataversion != nil {
		dvIn = append(dvIn, *filter.Dataversion)
	}
	dvIn = append(dvIn, filter.Dataversions...)
	if len(dvIn) > 0 {
		opts.DataVersion = &grpc.StringFilterOption{In: dvIn}
	}
	if filter.Source != nil {
		opts.Source = &grpc.StringFilterOption{In: []string{*filter.Source}}
	}
	if filter.Producer != nil {
		opts.Producer = &grpc.StringFilterOption{In: []string{*filter.Producer}}
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
	idx.Tags = grpc.TagsOrEmpty(idx.Tags)
	return &model.CloudEventIndex{
		Header:   &idx.CloudEventHeader,
		IndexKey: idx.Data.Key,
	}
}
