package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/SonChegg/PyMax/protocol"
)

// LenientInt64 decodes from either a JSON number or a numeric JSON string,
// matching pydantic's lenient `int` field coercion (pymax declares fields
// like ConfirmRegistrationResponse.user_token as plain `int`, and pydantic
// silently accepts a numeric string there) — MAX's wire format isn't
// consistent about which encoding it uses for large IDs. An empty string
// decodes to 0.
type LenientInt64 int64

func (n *LenientInt64) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		if s == "" {
			*n = 0
			return nil
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("api: LenientInt64: %w", err)
		}
		*n = LenientInt64(v)
		return nil
	}
	var v int64
	if err := json.Unmarshal(trimmed, &v); err != nil {
		return err
	}
	*n = LenientInt64(v)
	return nil
}

// ErrMissingPayload mirrors pymax's PyMaxError("Missing payload in response").
var ErrMissingPayload = fmt.Errorf("api: missing payload in response")

// remarshal round-trips v through JSON into dst, standing in for pydantic's
// model_validate() on an already-decoded payload dict/value.
func remarshal(v any, dst any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// payloadItem returns response.Payload[key], or nil if absent.
func payloadItem(frame protocol.InboundFrame, key string) any {
	if frame.Payload == nil {
		return nil
	}
	return frame.Payload[key]
}

// requirePayloadItem returns response.Payload[key], erroring if absent.
func requirePayloadItem(frame protocol.InboundFrame, key string) (any, error) {
	v := payloadItem(frame, key)
	if v == nil {
		return nil, fmt.Errorf("api: missing `%s` in response", key)
	}
	return v, nil
}

// decodeModel parses frame.Payload as a T, a port of pymax's parse_payload_model.
func decodeModel[T any](frame protocol.InboundFrame) (*T, error) {
	if frame.Payload == nil {
		return nil, nil
	}
	var v T
	if err := remarshal(frame.Payload, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// requireModel parses frame.Payload as a T, erroring on an empty payload, a
// port of pymax's require_payload_model.
func requireModel[T any](frame protocol.InboundFrame) (T, error) {
	var zero T
	if frame.Payload == nil {
		return zero, ErrMissingPayload
	}
	var v T
	if err := remarshal(frame.Payload, &v); err != nil {
		return zero, err
	}
	return v, nil
}

// decodeItemModel parses frame.Payload[key] as a T, a port of pymax's
// parse_payload_item_model.
func decodeItemModel[T any](frame protocol.InboundFrame, key string) (*T, error) {
	item := payloadItem(frame, key)
	if item == nil {
		return nil, nil
	}
	var v T
	if err := remarshal(item, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// requireItemModel parses frame.Payload[key] as a T, erroring if absent, a
// port of pymax's require_payload_item_model.
func requireItemModel[T any](frame protocol.InboundFrame, key string) (T, error) {
	var zero T
	item, err := requirePayloadItem(frame, key)
	if err != nil {
		return zero, err
	}
	var v T
	if err := remarshal(item, &v); err != nil {
		return zero, err
	}
	return v, nil
}

// decodeList parses frame.Payload[key] as a []T, a port of pymax's
// parse_payload_list.
func decodeList[T any](frame protocol.InboundFrame, key string) ([]T, error) {
	item := payloadItem(frame, key)
	if item == nil {
		return nil, nil
	}
	var v []T
	if err := remarshal(item, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// toPayload marshals v (a payload struct) to a map[string]any suitable for
// InvokeFunc, a port of pymax's CamelModel.to_payload (model_dump with
// by_alias=True, exclude_none=True). Struct fields must already carry the
// right JSON tags (required fields with no omitempty, optional fields as
// pointers/slices/maps with omitempty) for the exclude_none semantics to
// match.
func toPayload(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{}
	}
	return m
}
