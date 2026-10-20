package lithium

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"

	"go.xchunk.org/lithium/internal"
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
// with ok=false, the returned error is an [*Error].
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

// CallRaw sends a pre-encoded request body as is and returns the raw result.
// It is the way to send bodies other than JSON that
// [Client.Upload] does not cover.
//
// The request must be non-nil and have a non-empty method and content type.
// If the Bot API responds with ok=false, the returned error is an [*Error].
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

// unwrap turns a transport result into a [Response]: a transport
// error is passed through and a non-ok envelope becomes an [*Error].
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

// Do calls method with payload encoded as JSON and decodes the result into Resp.
//
//	ok, err := client.Do[bool](ctx, "deleteMessage", DeleteMessage{...})
//
// Errors of [Client.Call] are passed through; a result that does not fit Resp
// is reported as a decoding error.
func (c *Client) Do[Resp any](ctx context.Context, method string, payload any) (Resp, error) {
	return decode[Resp](c.Call(ctx, NewRequest(method, payload)))
}

// Send is like [Client.Do], but takes the method name from req and uses req
// itself as the payload. A nil req is reported as an error.
func (c *Client) Send[Resp any](ctx context.Context, req Method) (Resp, error) {
	if req == nil {
		return zero[Resp](errors.New("request must not be empty"))
	}
	return decode[Resp](c.Call(ctx, NewRequest(req.Method(), req)))
}

// Upload is like [Client.Send], but sends req as multipart/form-data along with files,
// as required by methods that upload files, e.g. sendPhoto or sendDocument.
//
//	msg, err := client.Upload[Message](ctx, SendPhoto{ChatID: 1},
//		lithium.File{Field: "photo", Name: "cat.jpg", Reader: f},
//	)
//
// Fields of req are encoded through its json tags: strings are sent as is, nulls
// are skipped and other values are sent as JSON. A file must not reuse the name of
// a field set by req. The whole body is built in memory before sending; use
// [Client.UploadStream] for large files.
func (c *Client) Upload[Resp any](ctx context.Context, req Method, files ...File) (Resp, error) {
	f, err := newForm(req, files)
	if err != nil {
		return zero[Resp](err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := f.write(mw); err != nil {
		return zero[Resp](err)
	}
	return decode[Resp](c.CallRaw(ctx, NewRawRequest(f.method, mw.FormDataContentType(), body.Bytes())))
}

// UploadStream is like [Client.Upload], but streams the body to the Bot API while
// reading the files instead of building it in memory first. The request is sent
// with chunked encoding. Files are no longer read once UploadStream returns.
func (c *Client) UploadStream[Resp any](ctx context.Context, req Method, files ...File) (Resp, error) {
	f, err := newForm(req, files)
	if err != nil {
		return zero[Resp](err)
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	done := make(chan struct{})
	go func() {
		defer close(done)
		pw.CloseWithError(f.write(mw))
	}()

	env, err := c.transport.DoReader(ctx, f.method, mw.FormDataContentType(), pr)
	// The transport may stop reading early, e.g. on a network error;
	// unblock the writer and wait for it to stop using the files.
	pr.Close()
	<-done
	return decode[Resp](unwrap(env, err))
}

// decode unmarshals the result of r into a new T. An error from the call
// producing r is passed through as is.
func decode[T any](r *Response, err error) (T, error) {
	var v T
	if err != nil {
		return v, err
	}
	if err := r.Decode(&v); err != nil {
		return v, err
	}
	return v, nil
}

// zero returns the zero value of T along with err.
func zero[T any](err error) (T, error) {
	var v T
	return v, err
}
