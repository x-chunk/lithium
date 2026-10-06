package lithium

import (
	"context"
	"errors"
)

type Client struct{}

func (c *Client) Call(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errors.New("request must not be empty")
	} else if req.method == "" {
		return nil, errors.New("method must not be empty")
	} else if req.payload == nil {
		return nil, errors.New("payload must not be empty")
	}

	//return internal.Call(ctx, req.method, req.payload)

	return nil, nil
}
