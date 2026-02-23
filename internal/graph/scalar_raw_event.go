package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/99designs/gqlgen/graphql"
	"github.com/DIMO-Network/cloudevent"
	"github.com/vektah/gqlparser/v2/ast"
)

// unmarshalInputRawEvent unmarshals RawEvent scalar from input (used when RawEvent appears as input; not used by current schema).
func (ec *executionContext) unmarshalInputRawEvent(ctx context.Context, v any) (cloudevent.CloudEvent[json.RawMessage], error) {
	if v == nil {
		return cloudevent.CloudEvent[json.RawMessage]{}, fmt.Errorf("RawEvent input is nil")
	}
	var data []byte
	switch u := v.(type) {
	case []byte:
		data = u
	case json.RawMessage:
		data = u
	default:
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return cloudevent.CloudEvent[json.RawMessage]{}, err
		}
	}
	var out cloudevent.CloudEvent[json.RawMessage]
	if err := json.Unmarshal(data, &out); err != nil {
		return cloudevent.CloudEvent[json.RawMessage]{}, err
	}
	return out, nil
}

// _RawEvent marshals the RawEvent scalar (CloudEvent payload) for output.
func (ec *executionContext) _RawEvent(ctx context.Context, sel ast.SelectionSet, v *cloudevent.CloudEvent[json.RawMessage]) graphql.Marshaler {
	if v == nil {
		return graphql.Null
	}
	return graphql.WriterFunc(func(w io.Writer) {
		data, err := json.Marshal(v)
		if err != nil {
			return
		}
		_, _ = w.Write(data)
	})
}
