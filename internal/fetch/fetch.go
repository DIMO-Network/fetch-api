// Package fetch provides functions for fetching objects from the index service.
package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/DIMO-Network/model-garage/pkg/cloudevent"
	"github.com/DIMO-Network/nameindexer/pkg/clickhouse/indexrepo"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"golang.org/x/sync/errgroup"
)

const fetchers = 25

// ListCloudEventsFromIndexes fetches a list of cloud events from the index service by trying to get them from each bucket in the list returning the first successful result.
func ListCloudEventsFromIndexes(ctx context.Context, idxSvc *indexrepo.Service, indexKeys []cloudevent.CloudEvent[indexrepo.ObjectInfo], buckets []string) ([]cloudevent.CloudEvent[json.RawMessage], error) {
	dataObjects := make([]cloudevent.CloudEvent[json.RawMessage], len(indexKeys))
	objectsByKeys := map[string]json.RawMessage{}
	// mutex to protect concurrent access to the map
	var mutex sync.RWMutex
	// create an error group to handle concurrent fetching
	group, errCtx := errgroup.WithContext(ctx)
	group.SetLimit(fetchers)

	for i, objectInfo := range indexKeys {
		// check if we already have this object before spawning a goroutine
		mutex.Lock()
		obj, alreadyFetched := objectsByKeys[objectInfo.Data.Key]
		if alreadyFetched {
			event := cloudevent.CloudEvent[json.RawMessage]{CloudEventHeader: objectInfo.CloudEventHeader, Data: obj}
			dataObjects[i] = event
			mutex.Unlock()
			continue
		}
		// mark the object as being fetched
		objectsByKeys[objectInfo.Data.Key] = nil
		mutex.Unlock()

		group.Go(func() error {
			obj, err := GetCloudEventFromIndex(errCtx, idxSvc, objectInfo, buckets)
			if err != nil {
				return fmt.Errorf("failed to get object: %w", err)
			}
			// there is a chance that 2 fetchers will try to fetch the same object
			mutex.Lock()
			objectsByKeys[objectInfo.Data.Key] = obj.Data
			dataObjects[i] = obj
			mutex.Unlock()

			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := group.Wait(); err != nil {
		return nil, err
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
