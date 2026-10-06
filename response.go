package lithium

import "encoding/json"

// Response holds the raw "result" field of a successful Bot API call.
// Its shape depends on the method, so decoding is left to the caller.
type Response struct {
	payload json.RawMessage
}

// Bytes returns the raw JSON of the result. The slice must not be modified.
func (r *Response) Bytes() []byte {
	return r.payload
}

// Decode unmarshals the result into v.
func (r *Response) Decode(v any) error {
	return json.Unmarshal(r.payload, v)
}
