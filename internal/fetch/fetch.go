// Package fetch provides functions for fetching objects from the index service.
package fetch

import (
	"context"
	"errors"
	"fmt"

	"github.com/DIMO-Network/nameindexer/pkg/clickhouse/indexrepo"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// GetObjectsFromIndexs gets objects from the index service by trying to get them from each bucket in the list returning the first successful result.
func GetObjectsFromIndexs(ctx context.Context, idxSvc *indexrepo.Service, indexKeys []string, buckets []string) ([]indexrepo.DataObject, error) {
	dataObjects := make([]indexrepo.DataObject, 0, len(indexKeys))
	for _, key := range indexKeys {
		obj, err := GetObjectFromIndex(ctx, idxSvc, key, buckets)
		if err != nil {
			return nil, fmt.Errorf("failed to get object: %w", err)
		}
		dataObjects = append(dataObjects, obj)
	}
	return dataObjects, nil
}

// GetObjectFromIndex gets an object from the index service by trying to get it from each bucket in the list returning the first successful result.
func GetObjectFromIndex(ctx context.Context, idxSvc *indexrepo.Service, indexKeys string, buckets []string) (indexrepo.DataObject, error) {
	var obj indexrepo.DataObject
	var err error
	// Try to get the object from each bucket in the list
	for _, bucket := range buckets {
		obj, err = idxSvc.GetObjectFromIndex(ctx, indexKeys, bucket)
		if err != nil {
			notFoundErr := &types.NoSuchKey{}
			if errors.As(err, &notFoundErr) {
				continue
			}
			return indexrepo.DataObject{}, fmt.Errorf("failed to get object: %w", err)
		}
		return obj, nil
	}
	return indexrepo.DataObject{}, fmt.Errorf("failed to get object: %w", err)
}
