package graph

import (
	"context"
	"testing"

	"github.com/DIMO-Network/cloudevent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDataUrlResolver verifies the DataUrl field resolver reads CloudEventWrapper.DataURL.
func TestDataUrlResolver(t *testing.T) {
	r := &cloudEventResolver{&Resolver{}}
	ctx := context.Background()

	t.Run("returns nil when DataURL is empty", func(t *testing.T) {
		obj := &CloudEventWrapper{Raw: &cloudevent.RawEvent{}}
		result, err := r.DataUrl(ctx, obj)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns URL when DataURL is set", func(t *testing.T) {
		const url = "https://s3.amazonaws.com/bucket/cloudevent/blobs/scan.bin?X-Amz-Signature=abc"
		obj := &CloudEventWrapper{
			Raw:     &cloudevent.RawEvent{},
			DataURL: url,
		}
		result, err := r.DataUrl(ctx, obj)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, url, *result)
	})

	t.Run("returns nil for nil wrapper", func(t *testing.T) {
		result, err := r.DataUrl(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

// TestBlobWrapperFieldsAreNil verifies that a CloudEventWrapper constructed for a
// blob (no inline payload) returns nil from the data and dataBase64 resolvers.
// This mirrors what the query resolver does when it builds a blob wrapper.
func TestBlobWrapperFieldsAreNil(t *testing.T) {
	r := &cloudEventResolver{&Resolver{}}
	ctx := context.Background()

	blobWrapper := &CloudEventWrapper{
		Raw:     &cloudevent.RawEvent{CloudEventHeader: cloudevent.CloudEventHeader{ID: "evt-1"}},
		DataURL: "https://example.com/presigned",
	}

	data, err := r.Data(ctx, blobWrapper)
	require.NoError(t, err)
	assert.Nil(t, data, "data should be nil for a blob wrapper")

	b64, err := r.DataBase64(ctx, blobWrapper)
	require.NoError(t, err)
	assert.Nil(t, b64, "dataBase64 should be nil for a blob wrapper")

	url, err := r.DataUrl(ctx, blobWrapper)
	require.NoError(t, err)
	require.NotNil(t, url)
	assert.Equal(t, "https://example.com/presigned", *url)
}
