// Package fetch provides functions for fetching objects from the index service.
package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DIMO-Network/model-garage/pkg/cloudevent"
	"github.com/DIMO-Network/nameindexer/pkg/clickhouse/indexrepo"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// GetCloudEventsFromKeys gets objects from the index service by trying to get them from each bucket in the list returning the first successful result.
func GetCloudEventsFromKeys(ctx context.Context, idxSvc *indexrepo.Service, indexKeys []string, buckets []string) ([]cloudevent.CloudEvent[json.RawMessage], error) {
	dataObjects := make([]cloudevent.CloudEvent[json.RawMessage], 0, len(indexKeys))
	for _, key := range indexKeys {
		obj, err := GetCloudEventFromKey(ctx, idxSvc, key, buckets)
		if err != nil {
			return nil, fmt.Errorf("failed to get object: %w", err)
		}
		dataObjects = append(dataObjects, obj)
	}
	return dataObjects, nil
}

// GetObjectsFromIndexs gets objects from the index service by trying to get them from each bucket in the list returning the first successful result.
func ListCloudEventsFromMetadata(ctx context.Context, idxSvc *indexrepo.Service, metadataList []indexrepo.CloudEventMetadata, buckets []string) ([]cloudevent.CloudEvent[json.RawMessage], error) {
	dataObjects := make([]cloudevent.CloudEvent[json.RawMessage], 0, len(metadataList))
	for _, metaData := range metadataList {
		obj, err := GetCloudEventFromKey(ctx, idxSvc, metaData.Key, buckets)
		if err != nil {
			return nil, fmt.Errorf("failed to get object: %w", err)
		}
		dataObjects = append(dataObjects, obj)
	}
	return dataObjects, nil
}

// GetCloudEventFromKey gets an object from the index service by trying to get it from each bucket in the list returning the first successful result.
func GetCloudEventFromKey(ctx context.Context, idxSvc *indexrepo.Service, indexKeys string, buckets []string) (cloudevent.CloudEvent[json.RawMessage], error) {
	var obj cloudevent.CloudEvent[json.RawMessage]
	var err error
	// Try to get the object from each bucket in the list
	for _, bucket := range buckets {
		obj, err = idxSvc.GetCloudEventFromKey(ctx, indexKeys, bucket)
		if err != nil {
			notFoundErr := &types.NoSuchKey{}
			if errors.As(err, &notFoundErr) {
				continue
			}
			return cloudevent.CloudEvent[json.RawMessage]{}, fmt.Errorf("failed to get object: %w", err)
		}
		return obj, nil
	}
	return cloudevent.CloudEvent[json.RawMessage]{}, fmt.Errorf("failed to get object: %w", err)
}
