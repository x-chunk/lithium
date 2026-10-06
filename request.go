package lithium

type request struct {
	method string
}

type Request struct {
	request
	payload any
}

type RawRequest struct {
	request
	payload []byte
}
