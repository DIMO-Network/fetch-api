package graph

import (
	"github.com/DIMO-Network/cloudevent"
)

// CloudEventWrapper holds a pointer to a RawEvent so resolvers can expose
// header, data, and dataBase64 without copying the underlying event.
type CloudEventWrapper struct {
	Raw *cloudevent.RawEvent
}
