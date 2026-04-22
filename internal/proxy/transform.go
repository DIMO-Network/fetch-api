package proxy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/graph/model"
)

// cloudEventHeaderFields is the GraphQL selection set for CloudEventHeader, shared across queries.
const cloudEventHeaderFields = `header {
	specversion type source subject id time
	datacontenttype dataschema dataversion producer signature raweventid tags
}`

// ProxiedCloudEvent holds a cloud event returned from dq, in fetch-api-native types.
// This avoids a circular import between the proxy and graph packages.
type ProxiedCloudEvent struct {
	Header     cloudevent.CloudEventHeader
	Data       []byte // raw JSON payload; nil when absent
	DataBase64 string
	DataURL    string
}

// ProxiedCloudIndex holds a cloud event header returned from dq for index-style queries.
type ProxiedCloudIndex struct {
	Header cloudevent.CloudEventHeader
}

// dqCloudEventJSON is an intermediate struct for unmarshaling dq cloud event JSON.
// dq uses camelCase field names (dataBase64, dataUrl) that differ from the cloudevent
// library's wire format (data_base64), so we cannot unmarshal directly into RawEvent.
type dqCloudEventJSON struct {
	Header     cloudevent.CloudEventHeader `json:"header"`
	Data       json.RawMessage             `json:"data"`
	DataBase64 string                      `json:"dataBase64"`
	DataUrl    string                      `json:"dataUrl"`
}

func dqJSONToProxied(ce dqCloudEventJSON) ProxiedCloudEvent {
	if ce.Header.Tags == nil {
		ce.Header.Tags = []string{}
	}
	var data []byte
	if len(ce.Data) > 0 && string(ce.Data) != "null" {
		data = ce.Data
	}
	return ProxiedCloudEvent{
		Header:     ce.Header,
		Data:       data,
		DataBase64: ce.DataBase64,
		DataURL:    ce.DataUrl,
	}
}

func dqJSONToProxiedIndex(ce dqCloudEventJSON) ProxiedCloudIndex {
	if ce.Header.Tags == nil {
		ce.Header.Tags = []string{}
	}
	return ProxiedCloudIndex{Header: ce.Header}
}

// BuildLatestCloudEventQuery constructs the dq query for latestCloudEvent, requesting all payload fields.
func BuildLatestCloudEventQuery() string {
	return `query LatestCloudEvent($subject: String!, $filter: CloudEventFilter) {
		latestCloudEvent(subject: $subject, filter: $filter) {
			` + cloudEventHeaderFields + `
			data dataBase64 dataUrl
		}
	}`
}

// BuildCloudEventsQuery constructs the dq query for cloudEvents, requesting all payload fields.
func BuildCloudEventsQuery() string {
	return `query CloudEvents($subject: String!, $limit: Int, $filter: CloudEventFilter) {
		cloudEvents(subject: $subject, limit: $limit, filter: $filter) {
			` + cloudEventHeaderFields + `
			data dataBase64 dataUrl
		}
	}`
}

// BuildLatestIndexQuery maps latestIndex to dq's latestCloudEvent with header fields only,
// avoiding an unnecessary S3 fetch since the index queries return metadata only.
func BuildLatestIndexQuery() string {
	return `query LatestIndex($subject: String!, $filter: CloudEventFilter) {
		latestCloudEvent(subject: $subject, filter: $filter) {
			` + cloudEventHeaderFields + `
		}
	}`
}

// BuildIndexesQuery maps indexes to dq's cloudEvents with header fields only.
func BuildIndexesQuery() string {
	return `query Indexes($subject: String!, $limit: Int, $filter: CloudEventFilter) {
		cloudEvents(subject: $subject, limit: $limit, filter: $filter) {
			` + cloudEventHeaderFields + `
		}
	}`
}

// BuildAvailableCloudEventTypesQuery constructs the dq query for availableCloudEventTypes.
func BuildAvailableCloudEventTypesQuery() string {
	return `query AvailableCloudEventTypes($subject: String!, $filter: CloudEventFilter) {
		availableCloudEventTypes(subject: $subject, filter: $filter) {
			type count firstSeen lastSeen
		}
	}`
}

// ProxyLatestCloudEvent forwards latestCloudEvent to dq and returns the result.
func (c *Client) ProxyLatestCloudEvent(ctx context.Context, subject string, filter *model.CloudEventFilter) (ProxiedCloudEvent, error) {
	data, err := c.Execute(ctx, BuildLatestCloudEventQuery(), map[string]any{
		"subject": subject,
		"filter":  filter,
	})
	if err != nil {
		return ProxiedCloudEvent{}, err
	}
	var envelope struct {
		LatestCloudEvent dqCloudEventJSON `json:"latestCloudEvent"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ProxiedCloudEvent{}, fmt.Errorf("unmarshaling latestCloudEvent: %w", err)
	}
	return dqJSONToProxied(envelope.LatestCloudEvent), nil
}

// ProxyCloudEvents forwards cloudEvents to dq and returns the results.
func (c *Client) ProxyCloudEvents(ctx context.Context, subject string, limit *int, filter *model.CloudEventFilter) ([]ProxiedCloudEvent, error) {
	data, err := c.Execute(ctx, BuildCloudEventsQuery(), map[string]any{
		"subject": subject,
		"limit":   limit,
		"filter":  filter,
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		CloudEvents []dqCloudEventJSON `json:"cloudEvents"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshaling cloudEvents: %w", err)
	}
	out := make([]ProxiedCloudEvent, len(envelope.CloudEvents))
	for i, ce := range envelope.CloudEvents {
		out[i] = dqJSONToProxied(ce)
	}
	return out, nil
}

// ProxyLatestIndex forwards latestIndex to dq via latestCloudEvent (header only) and returns the result.
func (c *Client) ProxyLatestIndex(ctx context.Context, subject string, filter *model.CloudEventFilter) (ProxiedCloudIndex, error) {
	data, err := c.Execute(ctx, BuildLatestIndexQuery(), map[string]any{
		"subject": subject,
		"filter":  filter,
	})
	if err != nil {
		return ProxiedCloudIndex{}, err
	}
	var envelope struct {
		LatestCloudEvent dqCloudEventJSON `json:"latestCloudEvent"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ProxiedCloudIndex{}, fmt.Errorf("unmarshaling latestCloudEvent (for latestIndex): %w", err)
	}
	return dqJSONToProxiedIndex(envelope.LatestCloudEvent), nil
}

// ProxyIndexes forwards indexes to dq via cloudEvents (header only) and returns the results.
func (c *Client) ProxyIndexes(ctx context.Context, subject string, limit *int, filter *model.CloudEventFilter) ([]ProxiedCloudIndex, error) {
	data, err := c.Execute(ctx, BuildIndexesQuery(), map[string]any{
		"subject": subject,
		"limit":   limit,
		"filter":  filter,
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		CloudEvents []dqCloudEventJSON `json:"cloudEvents"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshaling cloudEvents (for indexes): %w", err)
	}
	out := make([]ProxiedCloudIndex, len(envelope.CloudEvents))
	for i, ce := range envelope.CloudEvents {
		out[i] = dqJSONToProxiedIndex(ce)
	}
	return out, nil
}

// ProxyAvailableCloudEventTypes forwards availableCloudEventTypes to dq and returns the results.
func (c *Client) ProxyAvailableCloudEventTypes(ctx context.Context, subject string, filter *model.CloudEventFilter) ([]*model.CloudEventTypeSummary, error) {
	data, err := c.Execute(ctx, BuildAvailableCloudEventTypesQuery(), map[string]any{
		"subject": subject,
		"filter":  filter,
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		AvailableCloudEventTypes []*model.CloudEventTypeSummary `json:"availableCloudEventTypes"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshaling availableCloudEventTypes: %w", err)
	}
	return envelope.AvailableCloudEventTypes, nil
}

