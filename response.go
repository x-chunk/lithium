package lithium

import "encoding/json"

// Response holds the raw "result" field of a successful Bot API call.
type Response struct {
	payload json.RawMessage
}

// Bytes returns the raw JSON of the result.
func (r *Response) Bytes() []byte {
	return r.payload
}

// Decode unmarshals the result into v.
func (r *Response) Decode(v any) error {
	return json.Unmarshal(r.payload, v)
}
