package grpc

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/DIMO-Network/cloudevent"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

const (
	maxRandomHours  = 1000
	randStringMin   = 5
	randStringRange = 10
	maxMapEntries   = 3
	maxExtraParts   = 1000
)

// TestCloudEventProtoRoundTrip tests that a CloudEvent can be converted to Proto and back without losing data.
// This test uses reflection to fill all fields with random data, ensuring it works regardless of structure changes.
func TestCloudEventProtoRoundTrip(t *testing.T) {
	t.Parallel()

	// Create a random CloudEvent by filling all fields with random data
	originalEvent := cloudevent.CloudEvent[json.RawMessage]{
		CloudEventHeader: fillRandomCloudEventHeader(t),
		Data:             json.RawMessage(`{"random": "test data"}`),
	}

	// Convert CloudEvent to Proto
	protoEvent := CloudEventToProto(originalEvent)

	// Verify Proto was created
	require.NotNil(t, protoEvent)
	require.NotNil(t, protoEvent.GetHeader())

	// Convert back from Proto to CloudEvent
	convertedEvent := protoEvent.AsCloudEvent()

	// Verify the data is preserved through round-trip conversion using deep equals
	require.Empty(t, cmp.Diff(originalEvent.CloudEventHeader, convertedEvent.CloudEventHeader))
	require.Equal(t, originalEvent.Data, convertedEvent.Data)
}

// TestCloudEventHeaderProtoRoundTrip tests that a CloudEventHeader can be converted to Proto and back without losing data.
func TestCloudEventHeaderProtoRoundTrip(t *testing.T) {
	t.Parallel()

	// Create a random header by filling all fields with random data
	originalHeader := fillRandomCloudEventHeader(t)

	// Convert to Proto
	protoHeader := CloudEventHeaderToProto(&originalHeader)

	// Verify Proto was created
	require.NotNil(t, protoHeader)

	// Convert back from Proto
	convertedHeader := protoHeader.AsCloudEventHeader()

	// Verify all fields are preserved through round-trip conversion using deep equals
	require.Empty(t, cmp.Diff(originalHeader, convertedHeader))
}

// TestCloudEventProtoWithNilHeader tests that nil headers are handled correctly.
func TestCloudEventProtoWithNilHeader(t *testing.T) {
	t.Parallel()

	originalEvent := cloudevent.CloudEvent[json.RawMessage]{
		CloudEventHeader: cloudevent.CloudEventHeader{},
		Data:             json.RawMessage(`{"test": "data"}`),
	}

	protoEvent := CloudEventToProto(originalEvent)
	require.NotNil(t, protoEvent)

	convertedEvent := protoEvent.AsCloudEvent()
	require.Equal(t, originalEvent.Data, convertedEvent.Data)
}

// TestCloudEventHeaderProtoNil tests that CloudEventHeaderToProto handles nil input.
func TestCloudEventHeaderProtoNil(t *testing.T) {
	t.Parallel()

	protoHeader := CloudEventHeaderToProto(nil)
	require.Nil(t, protoHeader)
}

// fillRandomCloudEventHeader fills all fields of CloudEventHeader with random data using reflection.
// It panics if it encounters a type it doesn't know how to handle.
//
//nolint:gocognit,revive,cyclop
func fillRandomCloudEventHeader(t *testing.T) cloudevent.CloudEventHeader {
	t.Helper()

	header := cloudevent.CloudEventHeader{}
	headerVal := reflect.ValueOf(&header).Elem()

	for i := range headerVal.NumField() {
		field := headerVal.Field(i)
		if !field.CanSet() {
			continue
		}

		switch field.Type().Kind() {
		case reflect.String:
			field.SetString(randomString())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(int64(rand.Intn(maxExtraParts)))
		case reflect.Float32, reflect.Float64:
			field.SetFloat(rand.Float64())
		case reflect.Bool:
			field.SetBool(rand.Intn(2) == 0)
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				// String slice
				slice := make([]string, rand.Intn(maxMapEntries)+1)
				for j := range slice {
					slice[j] = randomString()
				}
				field.Set(reflect.ValueOf(slice))
			} else {
				panic("unsupported slice type: " + field.Type().String())
			}
		case reflect.Struct:
			// Handle time.Time - truncate to second precision (what protobuf preserves)
			if field.Type() == reflect.TypeOf(time.Time{}) {
				t := time.Now().Add(time.Duration(rand.Intn(maxRandomHours)) * time.Hour).Truncate(time.Second)
				field.Set(reflect.ValueOf(t))
			} else {
				panic("unsupported struct type: " + field.Type().String())
			}
		case reflect.Map:
			// Handle map[string]any
			if field.Type() == reflect.TypeOf(map[string]any{}) {
				extrasMap := make(map[string]any)
				for j := rand.Intn(maxMapEntries) + 1; j > 0; j-- {
					switch rand.Intn(2) {
					case 0:
						extrasMap[randomString()] = randomString()
					case 1:
						extrasMap[randomString()] = rand.Float64()
					}
				}
				field.Set(reflect.ValueOf(extrasMap))
			} else {
				panic("unsupported map type: " + field.Type().String())
			}
		default:
			panic("unsupported field type: " + field.Type().String())
		}
	}

	return header
}

// randomString generates a random string.
func randomString() string {
	length := rand.Intn(randStringRange) + randStringMin
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
