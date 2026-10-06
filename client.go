package lithium

import (
	"context"
	"errors"
	"net/http"

	"github.com/x-chunk/lithium/internal"
	"go.xchunk.org/anvil/v2/result"
)

type Client struct {
	transport internal.Transport
}

type Option func(*Client)

type Method interface {
	Method() string
}

// WithHTTPClient sets the HTTP client used for requests. http.DefaultClient is used by default.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.transport.HTTPClient = hc }
}

// WithBaseURL sets the Bot API server URL, e.g. for a local Bot API server.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.transport.BaseURL = url }
}

func New(token string, opts ...Option) *Client {
	c := &Client{transport: internal.Transport{Token: token}}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Call(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errors.New("request must not be empty")
	} else if req.method == "" {
		return nil, errors.New("method must not be empty")
	} else if req.payload == nil {
		return nil, errors.New("payload must not be empty")
	}

	env, err := c.transport.DoJSON(ctx, req.method, req.payload)
	return unwrap(env, err)
}

func (c *Client) CallRaw(ctx context.Context, req *RawRequest) (*Response, error) {
	if req == nil {
		return nil, errors.New("request must not be empty")
	} else if req.method == "" {
		return nil, errors.New("method must not be empty")
	} else if req.contentType == "" {
		return nil, errors.New("content type must not be empty")
	}

	env, err := c.transport.Do(ctx, req.method, req.contentType, req.payload)
	return unwrap(env, err)
}

func unwrap(env *internal.Envelope, err error) (*Response, error) {
	if err != nil {
		return nil, err
	}
	if !env.Ok {
		apiErr := &Error{Code: env.ErrorCode, Description: env.Description}
		if p := env.Parameters; p != nil {
			apiErr.RetryAfter = p.RetryAfter
			apiErr.MigrateToChatID = p.MigrateToChatID
		}
		return nil, apiErr
	}
	return &Response{payload: env.Result}, nil
}

func (c *Client) Do[Resp any](ctx context.Context, method string, payload any) result.Result[Resp] {
	return result.Of(c.Call(ctx, NewRequest(method, payload))).AndThen(decode[Resp])
}

func (c *Client) Send[Resp any](ctx context.Context, req Method) result.Result[Resp] {
	if req == nil {
		return result.Err[Resp](errors.New("request must not be empty"))
	}
	return result.Of(c.Call(ctx, NewRequest(req.Method(), req))).AndThen(decode[Resp])
}

func decode[T any](r *Response) result.Result[T] {
	var v T
	err := r.Decode(&v)
	return result.Of(v, err)
}
