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

// GetObjectsFromIndexs gets objects from the index service by trying to get them from each bucket in the list returning the first successful result.
func ListCloudEventsFromIndexes(ctx context.Context, idxSvc *indexrepo.Service, indexKeys []cloudevent.CloudEvent[indexrepo.ObjectInfo], buckets []string) ([]cloudevent.CloudEvent[json.RawMessage], error) {
	dataObjects := make([]cloudevent.CloudEvent[json.RawMessage], 0, len(indexKeys))
	objectsByKeys := map[string]json.RawMessage{}
	for _, objectInfo := range indexKeys {
		if obj, ok := objectsByKeys[objectInfo.Data.Key]; ok {
			event := cloudevent.CloudEvent[json.RawMessage]{CloudEventHeader: objectInfo.CloudEventHeader, Data: obj}
			dataObjects = append(dataObjects, event)
			continue
		}
		obj, err := GetCloudEventFromIndex(ctx, idxSvc, objectInfo, buckets)
		if err != nil {
			return nil, fmt.Errorf("failed to get object: %w", err)
		}
		objectsByKeys[objectInfo.Data.Key] = obj.Data
		dataObjects = append(dataObjects, obj)
	}
	return dataObjects, nil
}

// GetCloudEventFromKey gets an object from the index service by trying to get it from each bucket in the list returning the first successful result.
func GetCloudEventFromIndex(ctx context.Context, idxSvc *indexrepo.Service, indexKeys cloudevent.CloudEvent[indexrepo.ObjectInfo], buckets []string) (cloudevent.CloudEvent[json.RawMessage], error) {
	var obj cloudevent.CloudEvent[json.RawMessage]
	var err error
	// Try to get the object from each bucket in the list
	for _, bucket := range buckets {
		obj, err = idxSvc.GetCloudEventFromIndex(ctx, indexKeys, bucket)
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
