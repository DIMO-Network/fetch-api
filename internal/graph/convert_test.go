package graph

import (
	"testing"
	"time"

	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexToModel(t *testing.T) {
	t.Run("header bound directly from library type", func(t *testing.T) {
		idx := cloudevent.CloudEvent[eventrepo.ObjectInfo]{
			CloudEventHeader: cloudevent.CloudEventHeader{
				ID:              "test-id",
				Source:          "0xABC",
				Producer:        "prod",
				Subject:         "subj",
				Time:            time.Date(2026, 2, 17, 1, 0, 0, 0, time.UTC),
				Type:            "dimo.status",
				DataContentType: "application/json",
				DataVersion:     "r/v0/s",
				RawEventID:      "raw-event-123",
			},
			Data: eventrepo.ObjectInfo{Key: "s3://bucket/key"},
		}
		out := indexToModel(idx)

		require.NotNil(t, out.Header)
		assert.Equal(t, "test-id", out.Header.ID)
		assert.Equal(t, "0xABC", out.Header.Source)
		assert.Equal(t, "dimo.status", out.Header.Type)
		assert.Equal(t, "application/json", out.Header.DataContentType)
		assert.Equal(t, "r/v0/s", out.Header.DataVersion)
		assert.Equal(t, "raw-event-123", out.Header.RawEventID)
		assert.Equal(t, "s3://bucket/key", out.IndexKey)
	})

	t.Run("empty optional fields remain zero values", func(t *testing.T) {
		idx := cloudevent.CloudEvent[eventrepo.ObjectInfo]{
			CloudEventHeader: cloudevent.CloudEventHeader{
				ID:      "test-id",
				Source:  "src",
				Subject: "subj",
				Type:    "dimo.status",
			},
			Data: eventrepo.ObjectInfo{Key: "key"},
		}
		out := indexToModel(idx)

		assert.Empty(t, out.Header.DataContentType)
		assert.Empty(t, out.Header.DataVersion)
		assert.Empty(t, out.Header.Signature)
		assert.Empty(t, out.Header.RawEventID)
	})
}
