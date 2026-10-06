package lithium

import (
	"context"
	"errors"
	"net/http"

	"github.com/x-chunk/lithium/internal"
	"go.xchunk.org/anvil/v2/result"
)

// Client sends calls to the Telegram Bot API on behalf of a single bot.
// It is safe for concurrent use as long as the underlying [http.Client] is.
type Client struct {
	transport internal.Transport
}

// Option configures a [Client] created by [New].
type Option func(*Client)

// Method is implemented by request structs that know their Bot API method name.
// The struct itself is encoded as the JSON payload; methods are not serialized,
// so Method does not leak into the request body.
type Method interface {
	// Method returns the Bot API method name, e.g. "sendMessage".
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

// New creates a client for the bot with the given token.
// Without options it talks to https://api.telegram.org using [http.DefaultClient].
func New(token string, opts ...Option) *Client {
	c := &Client{transport: internal.Transport{Token: token}}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Call encodes the request payload as JSON, sends it and returns the raw result.
//
// The request must be non-nil and have a non-empty method and a non-nil payload;
// use an empty struct for methods without parameters. If the Bot API responds
// with ok=false, the error is an [*Error].
func (c *Client) Call(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, errors.New("request must not be empty")
	} else if req.method == "" {
		return nil, errors.New("method must not be empty")
	} else if req.payload == nil {
		return nil, errors.New("payload must not be empty")
	}

	env, err := c.transport.DoJSON(ctx, req.method, req.payload)
	return unwrap(env, err).Value()
}

// CallRaw sends a pre-encoded request body as is and returns the raw result.
// It is the way to send bodies other than JSON, e.g. multipart/form-data with files.
//
// The request must be non-nil and have a non-empty method and content type.
// If the Bot API responds with ok=false, the error is an [*Error].
func (c *Client) CallRaw(ctx context.Context, req *RawRequest) (*Response, error) {
	if req == nil {
		return nil, errors.New("request must not be empty")
	} else if req.method == "" {
		return nil, errors.New("method must not be empty")
	} else if req.contentType == "" {
		return nil, errors.New("content type must not be empty")
	}

	env, err := c.transport.Do(ctx, req.method, req.contentType, req.payload)
	return unwrap(env, err).Value()
}

// unwrap turns a transport result into a [result.Result] of [Response]: a transport
// error is passed through and a non-ok envelope becomes an [*Error].
func unwrap(env *internal.Envelope, err error) result.Result[*Response] {
	if err != nil {
		return result.Err[*Response](err)
	}
	if !env.Ok {
		apiErr := &Error{Code: env.ErrorCode, Description: env.Description}
		if p := env.Parameters; p != nil {
			apiErr.RetryAfter = p.RetryAfter
			apiErr.MigrateToChatID = p.MigrateToChatID
		}
		return result.Err[*Response](apiErr)
	}
	return result.Ok(&Response{payload: env.Result})
}

// Do calls method with payload encoded as JSON and decodes the result into Resp.
//
//	ok, err := client.Do[bool](ctx, "deleteMessage", DeleteMessage{...}).Value()
//
// Errors of [Client.Call] are passed through; a result that does not fit Resp
// is reported as a decoding error.
func (c *Client) Do[Resp any](ctx context.Context, method string, payload any) result.Result[Resp] {
	return result.Of(c.Call(ctx, NewRequest(method, payload))).AndThen(decode[Resp])
}

// Send is like [Client.Do], but takes the method name from req and uses req
// itself as the payload. A nil req is reported as an error.
func (c *Client) Send[Resp any](ctx context.Context, req Method) result.Result[Resp] {
	if req == nil {
		return result.Err[Resp](errors.New("request must not be empty"))
	}
	return result.Of(c.Call(ctx, NewRequest(req.Method(), req))).AndThen(decode[Resp])
}

// decode unmarshals the result of r into a new T.
func decode[T any](r *Response) result.Result[T] {
	var v T
	err := r.Decode(&v)
	return result.Of(v, err)
}
