//go:generate go tool mockgen -source=./eventrepo.go -destination=eventrepo_mock_test.go -package=eventrepo_test
package eventrepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"math/big"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	chconfig "github.com/DIMO-Network/clickhouse-infra/pkg/connect/config"
	"github.com/DIMO-Network/clickhouse-infra/pkg/container"
	"github.com/DIMO-Network/cloudevent"
	chindexer "github.com/DIMO-Network/cloudevent/pkg/clickhouse"
	"github.com/DIMO-Network/cloudevent/pkg/clickhouse/migrations"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var dataType = "small"

// setupClickHouseContainer starts a ClickHouse container for testing and returns the connection.
func setupClickHouseContainer(t *testing.T) *container.Container {
	ctx := context.Background()
	settings := chconfig.Settings{
		User:     "default",
		Database: "dimo",
	}

	chContainer, err := container.CreateClickHouseContainer(ctx, settings)
	require.NoError(t, err)

	chDB, err := chContainer.GetClickhouseAsDB()
	require.NoError(t, err)

	// Ensure we terminate the container at the end
	t.Cleanup(func() {
		chContainer.Terminate(ctx)
	})

	err = migrations.RunGoose(ctx, []string{"up"}, chDB)
	require.NoError(t, err)

	return chContainer
}

// insertTestData inserts test data into ClickHouse.
func insertTestData(t *testing.T, ctx context.Context, conn clickhouse.Conn, index *cloudevent.CloudEventHeader) string {
	values := chindexer.CloudEventToSlice(index)

	err := conn.Exec(ctx, chindexer.InsertStmt, values...)
	require.NoError(t, err)
	return values[len(values)-1].(string)
}

// TestGetLatestIndexKey tests the GetLatestIndexKey function.
func TestGetLatestIndexKey(t *testing.T) {
	chContainer := setupClickHouseContainer(t)

	// Insert test data
	conn, err := chContainer.GetClickHouseAsConn()
	require.NoError(t, err)
	contractAddr := randAddress()
	device1TokenID := big.NewInt(1234567890)
	device2TokenID := big.NewInt(976543210)
	ctx := context.Background()
	now := time.Now()

	// Create test indices
	eventIdx1 := &cloudevent.CloudEventHeader{
		Subject: cloudevent.ERC721DID{
			ChainID:         153,
			ContractAddress: contractAddr,
			TokenID:         device1TokenID,
		}.String(),
		DataVersion: dataType,
		Time:        now.Add(-1 * time.Hour),
	}

	eventIdx2 := &cloudevent.CloudEventHeader{
		Subject: cloudevent.ERC721DID{
			ChainID:         153,
			ContractAddress: contractAddr,
			TokenID:         device1TokenID,
		}.String(),
		DataVersion: dataType,
		Time:        now,
	}

	// Insert test data
	_ = insertTestData(t, ctx, conn, eventIdx1)
	indexKey2 := insertTestData(t, ctx, conn, eventIdx2)

	tests := []struct {
		name          string
		subject       cloudevent.ERC721DID
		expectedKey   string
		expectedError bool
	}{
		{
			name: "valid latest object",
			subject: cloudevent.ERC721DID{
				ChainID:         153,
				ContractAddress: contractAddr,
				TokenID:         device1TokenID,
			},
			expectedKey: indexKey2,
		},
		{
			name: "no records",
			subject: cloudevent.ERC721DID{
				ChainID:         153,
				ContractAddress: contractAddr,
				TokenID:         device2TokenID,
			},
			expectedError: true,
		},
	}

	indexService := eventrepo.New(conn, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &grpc.SearchOptions{
				DataVersion: &wrapperspb.StringValue{Value: dataType},
				Subject:     &wrapperspb.StringValue{Value: tt.subject.String()},
			}
			metadata, err := indexService.GetLatestIndex(context.Background(), opts)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedKey, metadata.Data.Key)
			}
		})
	}
}

// TestGetDataFromIndex tests the GetDataFromIndex function.
func TestGetDataFromIndex(t *testing.T) {
	chContainer := setupClickHouseContainer(t)
	contractAddr := randAddress()
	device1TokenID := big.NewInt(1234567890)
	device2TokenID := big.NewInt(976543210)

	conn, err := chContainer.GetClickHouseAsConn()
	require.NoError(t, err)
	ctx := context.Background()

	eventIdx := &cloudevent.CloudEventHeader{
		Subject: cloudevent.ERC721DID{
			ChainID:         153,
			ContractAddress: contractAddr,
			TokenID:         device1TokenID,
		}.String(),
		DataVersion: dataType,
		Time:        time.Now().Add(-1 * time.Hour),
	}

	_ = insertTestData(t, ctx, conn, eventIdx)

	tests := []struct {
		name            string
		subject         cloudevent.ERC721DID
		expectedContent []byte
		expectedError   bool
	}{
		{
			name: "valid object content",
			subject: cloudevent.ERC721DID{
				ChainID:         153,
				ContractAddress: contractAddr,
				TokenID:         device1TokenID,
			},
			expectedContent: []byte(`{"vin": "1HGCM82633A123456"}`),
		},
		{
			name: "no records",
			subject: cloudevent.ERC721DID{
				ChainID:         153,
				ContractAddress: contractAddr,
				TokenID:         device2TokenID,
			},
			expectedError: true,
		},
	}

	ctrl := gomock.NewController(t)
	mockS3Client := NewMockObjectGetter(ctrl)
	content := []byte(`{"vin": "1HGCM82633A123456"}`)
	mockS3Client.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(content)),
		ContentLength: ref(int64(len(content))),
	}, nil).AnyTimes()

	indexService := eventrepo.New(conn, mockS3Client)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &grpc.SearchOptions{
				DataVersion: &wrapperspb.StringValue{Value: dataType},
				Subject:     &wrapperspb.StringValue{Value: tt.subject.String()},
			}
			content, err := indexService.GetLatestCloudEvent(context.Background(), "test-bucket", opts)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.EqualValues(t, tt.expectedContent, []byte(content.Data))
			}
		})
	}
}

func TestStoreObject(t *testing.T) {
	chContainer := setupClickHouseContainer(t)

	conn, err := chContainer.GetClickHouseAsConn()
	require.NoError(t, err)
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	mockS3Client := NewMockObjectGetter(ctrl)
	mockS3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil).AnyTimes()

	indexService := eventrepo.New(conn, mockS3Client)

	content := []byte(`{"vin": "1HGCM82633A123456"}`)
	did := cloudevent.ERC721DID{
		ChainID:         153,
		ContractAddress: randAddress(),
		TokenID:         big.NewInt(123456),
	}

	event := cloudevent.CloudEvent[json.RawMessage]{
		CloudEventHeader: cloudevent.CloudEventHeader{
			Subject:     did.String(),
			Time:        time.Now(),
			DataVersion: dataType,
		},
		Data: content,
	}
	err = indexService.StoreObject(ctx, "test-bucket", &event.CloudEventHeader, event.Data)
	require.NoError(t, err)

	// Verify the data is stored in ClickHouse
	opts := &grpc.SearchOptions{
		DataVersion: &wrapperspb.StringValue{Value: dataType},
		Subject:     &wrapperspb.StringValue{Value: did.String()},
	}
	metadata, err := indexService.GetLatestIndex(ctx, opts)
	require.NoError(t, err)
	expectedIndexKey := chindexer.CloudEventToObjectKey(&event.CloudEventHeader)
	require.NoError(t, err)
	require.Equal(t, expectedIndexKey, metadata.Data.Key)
}

// TestGetData tests the GetData function with different SearchOptions combinations.
func TestGetData(t *testing.T) {
	chContainer := setupClickHouseContainer(t)

	// Insert test data
	conn, err := chContainer.GetClickHouseAsConn()
	require.NoError(t, err)
	source1 := randAddress()
	contractAddr := randAddress()
	device1TokenID := big.NewInt(123456)
	device2TokenID := big.NewInt(654321)
	ctx := context.Background()
	now := time.Now()
	eventDID := cloudevent.ERC721DID{
		ChainID:         153,
		ContractAddress: contractAddr,
		TokenID:         device1TokenID,
	}
	eventIdx := cloudevent.CloudEventHeader{
		Subject: eventDID.String(),
		Time:    now.Add(-4 * time.Hour),
		Producer: cloudevent.ERC721DID{
			ChainID:         153,
			ContractAddress: contractAddr,
			TokenID:         device1TokenID,
		}.String(),
		Type:        cloudevent.TypeStatus,
		Source:      source1.Hex(),
		DataVersion: dataType,
	}
	indexKey1 := insertTestData(t, ctx, conn, &eventIdx)
	eventIdx2 := eventIdx
	eventIdx2.Time = now.Add(-3 * time.Hour)
	indexKey2 := insertTestData(t, ctx, conn, &eventIdx2)
	eventIdx3 := eventIdx
	eventIdx3.Time = now.Add(-2 * time.Hour)
	eventIdx3.Type = cloudevent.TypeFingerprint
	indexKey3 := insertTestData(t, ctx, conn, &eventIdx3)
	eventIdx4 := eventIdx
	eventIdx4.Time = now.Add(-1 * time.Hour)
	eventIdx4.DataContentType = "utf-8"
	indexKey4 := insertTestData(t, ctx, conn, &eventIdx4)

	tests := []struct {
		name              string
		opts              *grpc.SearchOptions
		expectedIndexKeys []string
		expectedError     bool
	}{
		{
			name: "valid data with address",
			opts: &grpc.SearchOptions{
				DataVersion: &wrapperspb.StringValue{Value: dataType},
				Subject:     &wrapperspb.StringValue{Value: eventDID.String()},
			},
			expectedIndexKeys: []string{indexKey4, indexKey3, indexKey2, indexKey1},
		},
		{
			name: "no records with address",
			opts: &grpc.SearchOptions{
				DataVersion: &wrapperspb.StringValue{Value: dataType},
				Subject: &wrapperspb.StringValue{Value: cloudevent.ERC721DID{
					ChainID:         153,
					ContractAddress: contractAddr,
					TokenID:         device2TokenID,
				}.String()},
			},
			expectedIndexKeys: nil,
			expectedError:     true,
		},
		{
			name: "data within time range",
			opts: &grpc.SearchOptions{
				DataVersion: &wrapperspb.StringValue{Value: dataType},
				After:       &timestamppb.Timestamp{Seconds: now.Add(-3 * time.Hour).Unix()},
				Before:      &timestamppb.Timestamp{Seconds: now.Add(-1 * time.Minute).Unix()},
			},
			expectedIndexKeys: []string{indexKey4, indexKey3},
		},
		{
			name: "data with type filter",
			opts: &grpc.SearchOptions{
				DataVersion: &wrapperspb.StringValue{Value: dataType},
				Type:        &wrapperspb.StringValue{Value: cloudevent.TypeStatus},
			},
			expectedIndexKeys: []string{indexKey4, indexKey2, indexKey1},
		},
		{
			name:              "data with nil options",
			opts:              nil,
			expectedIndexKeys: []string{indexKey4, indexKey3, indexKey2, indexKey1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockS3Client := NewMockObjectGetter(ctrl)

			indexService := eventrepo.New(conn, mockS3Client)
			var expectedContent [][]byte
			for _, indexKey := range tt.expectedIndexKeys {
				mockS3Client.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					require.Equal(t, *params.Key, indexKey)
					quotedKey := `"` + indexKey + `"`
					// content := []byte(`{"data":` + quotedKey + `}`)
					expectedContent = append(expectedContent, []byte(quotedKey))
					return &s3.GetObjectOutput{
						Body:          io.NopCloser(bytes.NewReader([]byte(quotedKey))),
						ContentLength: ref(int64(len(quotedKey))),
					}, nil
				})
			}
			events, err := indexService.ListCloudEvents(context.Background(), "test-bucket", 10, tt.opts)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, events, len(expectedContent))
				for i, content := range expectedContent {
					require.Equal(t, string(content), string(events[i].Data))
				}
			}
		})
	}
}

// TestGetEventWithAllHeaderFields tests retrieving events with all header fields properly populated
func TestGetEventWithAllHeaderFields(t *testing.T) {
	chContainer := setupClickHouseContainer(t)

	conn, err := chContainer.GetClickHouseAsConn()
	require.NoError(t, err)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Create a DID for the test
	contractAddr := randAddress()
	deviceTokenID := big.NewInt(1234567890)
	eventDID1 := cloudevent.ERC721DID{
		ChainID:         153,
		ContractAddress: contractAddr,
		TokenID:         deviceTokenID,
	}
	eventDID2 := cloudevent.ERC721DID{
		ChainID:         151,
		ContractAddress: contractAddr,
		TokenID:         deviceTokenID,
	}

	// Create event with all header fields populated
	fullHeaderEvent := cloudevent.CloudEventHeader{
		ID:              "test-id-123456",
		Source:          "test-source",
		Producer:        "test-producer",
		Subject:         eventDID1.String(),
		Time:            now,
		Type:            cloudevent.TypeStatus,
		DataContentType: "application/json",
		DataSchema:      "https://example.com/schemas/status.json",
		DataVersion:     dataType,
		SpecVersion:     cloudevent.SpecVersion,
		Signature:       "0x1234567890",
		Tags:            []string{"tests.tag1", "tests.tag2"},
		Extras: map[string]any{
			"extraField": "extra-value",
		},
	}

	fullHeaderEvent2 := fullHeaderEvent
	fullHeaderEvent2.Subject = eventDID2.String()
	fullHeaderEvent2.Signature = ""
	fullHeaderEvent2.Extras = map[string]any{
		"signature": "0x09876543210",
	}

	// Insert the event
	indexKey := insertTestData(t, ctx, conn, &fullHeaderEvent)
	indexKey2 := insertTestData(t, ctx, conn, &fullHeaderEvent2)

	// Setup mock S3 client for retrieving the event data
	ctrl := gomock.NewController(t)
	mockS3Client := NewMockObjectGetter(ctrl)
	eventData := []byte(`{"status": "online", "lastSeen": "2023-01-01T12:00:00Z"}`)

	// Create service
	indexService := eventrepo.New(conn, mockS3Client)

	// Test retrieving the event
	t.Run("retrieve event with full headers", func(t *testing.T) {
		mockS3Client.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				// Verify the correct key was requested
				require.Equal(t, indexKey, *params.Key)
				return &s3.GetObjectOutput{
					Body:          io.NopCloser(bytes.NewReader(eventData)),
					ContentLength: ref(int64(len(eventData))),
				}, nil
			},
		)
		opts := &grpc.SearchOptions{
			DataVersion: &wrapperspb.StringValue{Value: dataType},
			Subject:     &wrapperspb.StringValue{Value: eventDID1.String()},
		}

		retrievedEvent, err := indexService.GetLatestCloudEvent(ctx, "test-bucket", opts)
		require.NoError(t, err)

		// Verify all header fields
		assert.Equal(t, fullHeaderEvent.ID, retrievedEvent.ID, "ID mismatch")
		assert.Equal(t, fullHeaderEvent.Source, retrievedEvent.Source, "Source mismatch")
		assert.Equal(t, fullHeaderEvent.Producer, retrievedEvent.Producer, "Producer mismatch")
		assert.Equal(t, fullHeaderEvent.Subject, retrievedEvent.Subject, "Subject mismatch")
		assert.Equal(t, fullHeaderEvent.Time.UTC().Truncate(time.Second), retrievedEvent.Time.UTC().Truncate(time.Second), "Time mismatch")
		assert.Equal(t, fullHeaderEvent.Type, retrievedEvent.Type, "Type mismatch")
		assert.Equal(t, fullHeaderEvent.DataContentType, retrievedEvent.DataContentType, "DataContentType mismatch")
		assert.Equal(t, fullHeaderEvent.DataSchema, retrievedEvent.DataSchema, "DataSchema mismatch")
		assert.Equal(t, fullHeaderEvent.DataVersion, retrievedEvent.DataVersion, "DataVersion mismatch")
		assert.Equal(t, cloudevent.SpecVersion, retrievedEvent.SpecVersion, "SpecVersion mismatch")
		assert.Equal(t, fullHeaderEvent.Signature, retrievedEvent.Signature, "Signature mismatch")
		assert.Equal(t, fullHeaderEvent.Tags, retrievedEvent.Tags, "Tags mismatch")

		// Verify extras
		require.NotNil(t, retrievedEvent.Extras)
		require.Equal(t, 2, len(retrievedEvent.Extras))
		require.Equal(t, "extra-value", retrievedEvent.Extras["extraField"])
		require.Equal(t, fullHeaderEvent.Signature, retrievedEvent.Extras["signature"].(string))

		// Verify data content
		require.Equal(t, string(eventData), string(retrievedEvent.Data))
	})

	t.Run("retrieve event with signature that is originally in extras", func(t *testing.T) {
		mockS3Client.EXPECT().GetObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				// Verify the correct key was requested
				require.Equal(t, indexKey2, *params.Key)
				return &s3.GetObjectOutput{
					Body:          io.NopCloser(bytes.NewReader(eventData)),
					ContentLength: ref(int64(len(eventData))),
				}, nil
			},
		)
		opts := &grpc.SearchOptions{
			DataVersion: &wrapperspb.StringValue{Value: dataType},
			Subject:     &wrapperspb.StringValue{Value: eventDID2.String()},
		}
		retrievedEvent, err := indexService.GetLatestCloudEvent(ctx, "test-bucket", opts)
		require.NoError(t, err)

		// Verify all header fields
		assert.Equal(t, fullHeaderEvent2.ID, retrievedEvent.ID, "ID mismatch")
		assert.Equal(t, fullHeaderEvent2.Extras["signature"], retrievedEvent.Signature, "Signature field not set correctly")
		assert.Equal(t, fullHeaderEvent2.Extras["signature"], retrievedEvent.Extras["signature"], "Signature field not set correctly in extras")
	})
}

func ref[T any](x T) *T {
	return &x
}

// TestListIndexesAdvanced tests the ListIndexesAdvanced function with various advanced filter options.
func TestListIndexesAdvanced(t *testing.T) {
	chContainer := setupClickHouseContainer(t)

	// Insert test data
	conn, err := chContainer.GetClickHouseAsConn()
	require.NoError(t, err)
	contractAddr := randAddress()
	device1TokenID := big.NewInt(1234567890)
	device2TokenID := big.NewInt(976543210)
	ctx := context.Background()
	now := time.Now()

	// Create test events with different characteristics for filtering
	eventIdx1 := &cloudevent.CloudEventHeader{
		ID: "event-source1-producer1",
		Subject: cloudevent.ERC721DID{
			ChainID:         153,
			ContractAddress: contractAddr,
			TokenID:         device1TokenID,
		}.String(),
		Type:        cloudevent.TypeStatus,
		Source:      "source-1",
		Producer:    "producer-1",
		DataVersion: "v1.0",
		Time:        now.Add(-3 * time.Hour),
		Tags:        []string{"vehicle", "status"},
	}

	eventIdx2 := &cloudevent.CloudEventHeader{
		ID: "event-source2-producer2",
		Subject: cloudevent.ERC721DID{
			ChainID:         153,
			ContractAddress: contractAddr,
			TokenID:         device1TokenID,
		}.String(),
		Type:        cloudevent.TypeFingerprint,
		Source:      "source-2",
		Producer:    "producer-2",
		DataVersion: "v2.0",
		Time:        now.Add(-2 * time.Hour),
		Tags:        []string{"security", "fingerprint"},
	}

	eventIdx3 := &cloudevent.CloudEventHeader{
		ID: "event-source1-producer3",
		Subject: cloudevent.ERC721DID{
			ChainID:         153,
			ContractAddress: contractAddr,
			TokenID:         device2TokenID,
		}.String(),
		Type:        cloudevent.TypeStatus,
		Source:      "source-1",
		Producer:    "producer-3",
		DataVersion: "v1.0",
		Time:        now.Add(-1 * time.Hour),
		Tags:        []string{"vehicle", "telemetry", "status"},
	}

	// Add an event with different tags for testing ArrayFilterOption
	eventIdx4 := &cloudevent.CloudEventHeader{
		ID: "event-source3-producer4",
		Subject: cloudevent.ERC721DID{
			ChainID:         153,
			ContractAddress: contractAddr,
			TokenID:         device1TokenID,
		}.String(),
		Type:        cloudevent.TypeStatus,
		Source:      "source-3",
		Producer:    "producer-4",
		DataVersion: "v1.0",
		Time:        now.Add(-30 * time.Minute),
		Tags:        []string{"telemetry", "realtime"},
	}

	// Insert test data
	keyTypeStatusSource1Producer1 := insertTestData(t, ctx, conn, eventIdx1)
	keyTypeFingerprintSource2Producer2 := insertTestData(t, ctx, conn, eventIdx2)
	keyTypeStatusSource1Producer3 := insertTestData(t, ctx, conn, eventIdx3)
	keyTypeStatusSource3Producer4 := insertTestData(t, ctx, conn, eventIdx4)

	indexService := eventrepo.New(conn, nil)

	tests := []struct {
		name              string
		advancedOpts      *grpc.AdvancedSearchOptions
		expectedIndexKeys []string
		expectedError     bool
	}{
		{
			name: "filter by single type status events",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Type: &grpc.StringFilterOption{
					HasAny: []string{cloudevent.TypeStatus},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource3Producer4, keyTypeStatusSource1Producer3, keyTypeStatusSource1Producer1},
		},
		{
			name: "filter by multiple types with OR logic",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Type: &grpc.StringFilterOption{
					HasAny: []string{cloudevent.TypeStatus, cloudevent.TypeFingerprint},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource3Producer4, keyTypeStatusSource1Producer3, keyTypeFingerprintSource2Producer2, keyTypeStatusSource1Producer1},
		},
		{
			name: "filter by type with negation",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Type: &grpc.StringFilterOption{
					HasAny: []string{cloudevent.TypeFingerprint},
					Negate: true,
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource3Producer4, keyTypeStatusSource1Producer3, keyTypeStatusSource1Producer1},
		},
		{
			name: "filter by source: multiple results",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Source: &grpc.StringFilterOption{
					HasAny: []string{"source-1"},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource1Producer3, keyTypeStatusSource1Producer1},
		},
		{
			name: "combine multiple filters (AND logic)",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Type: &grpc.StringFilterOption{
					HasAny: []string{cloudevent.TypeStatus},
				},
				Source: &grpc.StringFilterOption{
					HasAny: []string{"source-1"},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource1Producer3, keyTypeStatusSource1Producer1},
		},
		{
			name: "OR logic within StringFilterOption",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Type: &grpc.StringFilterOption{
					HasAny: []string{cloudevent.TypeAttestation},
					Or: &grpc.StringFilterOption{
						HasAny: []string{cloudevent.TypeStatus},
						Negate: true,
					},
				},
			},
			expectedIndexKeys: []string{keyTypeFingerprintSource2Producer2},
		},
		{
			name: "no matching records",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Type: &grpc.StringFilterOption{
					HasAny: []string{"non-existent-type"},
				},
			},
			expectedIndexKeys: []string{},
			expectedError:     true,
		},
		{
			name: "filter by tags: has any (telemetry)",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Tags: &grpc.ArrayFilterOption{
					HasAny: []string{"telemetry"},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource3Producer4, keyTypeStatusSource1Producer3},
		},
		{
			name: "filter by tags: has all (vehicle AND status)",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Tags: &grpc.ArrayFilterOption{
					HasAll: []string{"vehicle", "status"},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource1Producer3, keyTypeStatusSource1Producer1},
		},
		{
			name: "filter by tags: has any with multiple values",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Tags: &grpc.ArrayFilterOption{
					HasAny: []string{"security", "realtime"},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource3Producer4, keyTypeFingerprintSource2Producer2},
		},
		{
			name: "filter by tags: negated has_any",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Tags: &grpc.ArrayFilterOption{
					HasAny: []string{"vehicle"},
					Negate: true,
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource3Producer4, keyTypeFingerprintSource2Producer2},
		},
		{
			name: "complex tags filter: has_any OR has_all combination",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Tags: &grpc.ArrayFilterOption{
					HasAny: []string{"fingerprint"},
					Or: &grpc.ArrayFilterOption{
						HasAll: []string{"telemetry", "realtime"},
					},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource3Producer4, keyTypeFingerprintSource2Producer2},
		},
		{
			name: "complex tags filter: has_all with OR chain",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Tags: &grpc.ArrayFilterOption{
					HasAll: []string{"vehicle", "telemetry"},
					Or: &grpc.ArrayFilterOption{
						HasAny: []string{"security"},
					},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource1Producer3, keyTypeFingerprintSource2Producer2},
		},
		{
			name: "complex negated tags with multiple conditions",
			advancedOpts: &grpc.AdvancedSearchOptions{
				Tags: &grpc.ArrayFilterOption{
					HasAny: []string{"fingerprint", "telemetry"},
					Negate: true, // Does NOT have fingerprint or telemetry
					Or: &grpc.ArrayFilterOption{
						HasAll: []string{"telemetry", "status"},
					},
				},
			},
			expectedIndexKeys: []string{keyTypeStatusSource1Producer3, keyTypeStatusSource1Producer1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := indexService.ListIndexesAdvanced(t.Context(), 10, tt.advancedOpts)
			if tt.expectedError {
				require.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err, "Unexpected error: %v", err)
				require.Len(t, results, len(tt.expectedIndexKeys), "Number of results mismatch")

				// Verify each result matches expected index keys in order
				actualKeys := make([]string, len(results))
				for i, result := range results {
					actualKeys[i] = result.Data.Key
				}

				require.Equal(t, tt.expectedIndexKeys, actualKeys, "Index keys mismatch")
			}
		})
	}
}

func randAddress() common.Address {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}
	return crypto.PubkeyToAddress(privateKey.PublicKey)
}
