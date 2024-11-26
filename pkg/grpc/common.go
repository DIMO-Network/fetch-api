package grpc

import (
	"encoding/json"

	"github.com/DIMO-Network/model-garage/pkg/cloudevent"
)

// AsCloudEvent converts the CloudEvent to a cloudevent.CloudEvent[json.RawMessage].
func (c *CloudEvent) AsCloudEvent() cloudevent.CloudEvent[json.RawMessage] {
	return cloudevent.CloudEvent[json.RawMessage]{
		CloudEventHeader: c.GetHeader().AsCloudEventHeader(),
		Data:             c.GetData(),
	}
}

// AsCloudEventHeader converts the CloudEventHeader to a cloudevent.CloudEventHeader.
func (c *CloudEventHeader) AsCloudEventHeader() cloudevent.CloudEventHeader {
	extras := make(map[string]any, len(c.GetExtras()))
	for k, v := range c.GetExtras() {
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			val = v
		}
		extras[k] = val
	}
	return cloudevent.CloudEventHeader{
		ID:              c.GetId(),
		Source:          c.GetSource(),
		Producer:        c.GetProducer(),
		SpecVersion:     c.GetSpecVersion(),
		Subject:         c.GetSubject(),
		Time:            c.GetTime().AsTime(),
		Type:            c.GetType(),
		DataContentType: c.GetDataContentType(),
		DataSchema:      c.GetDataSchema(),
		DataVersion:     c.GetDataVersion(),
		Extras:          extras,
	}
}
