package lithium

type request struct {
	method string
}

// Request is a Bot API call whose payload is encoded as JSON.
type Request struct {
	request
	payload any
}

// NewRequest creates a call of method with payload encoded as JSON.
func NewRequest(method string, payload any) *Request {
	return &Request{request: request{method: method}, payload: payload}
}

// RawRequest is a Bot API call whose payload is sent as is.
type RawRequest struct {
	request
	contentType string
	payload     []byte
}

// NewRawRequest creates a call of method with a pre-encoded payload of the given content type,
// e.g. "application/json" or a multipart body with its boundary.
func NewRawRequest(method, contentType string, payload []byte) *RawRequest {
	return &RawRequest{request: request{method: method}, contentType: contentType, payload: payload}
}
