package lithium

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("TOKEN", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
}

func TestCallSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendMessage" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"chat_id":1,"text":"hi"}` {
			t.Errorf("body = %s", body)
		}
		io.WriteString(w, `{"ok":true,"result":{"message_id":42}}`)
	})

	resp, err := c.Call(context.Background(), NewRequest("sendMessage", map[string]any{"chat_id": 1, "text": "hi"}))
	if err != nil {
		t.Fatal(err)
	}
	var msg struct {
		MessageID int `json:"message_id"`
	}
	if err := resp.Decode(&msg); err != nil {
		t.Fatal(err)
	}
	if msg.MessageID != 42 {
		t.Errorf("message_id = %d", msg.MessageID)
	}
}

func TestCallAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":5}}`)
	})

	_, err := c.Call(context.Background(), NewRequest("getMe", struct{}{}))
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *Error", err)
	}
	if apiErr.Code != 429 || apiErr.RetryAfter != 5 {
		t.Errorf("err = %+v", apiErr)
	}
}

func TestCallRaw(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "text/plain" {
			t.Errorf("content type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "raw" {
			t.Errorf("body = %s", body)
		}
		io.WriteString(w, `{"ok":true,"result":true}`)
	})

	resp, err := c.CallRaw(context.Background(), NewRawRequest("x", "text/plain", []byte("raw")))
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Bytes()) != "true" {
		t.Errorf("result = %s", resp.Bytes())
	}
}

func TestTransportErrorHidesToken(t *testing.T) {
	c := New("SECRET", WithBaseURL("http://127.0.0.1:1"))
	_, err := c.Call(context.Background(), NewRequest("getMe", struct{}{}))
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error leaks token: %v", err)
	}
}

func TestCallValidation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent")
	})
	ctx := context.Background()

	tests := map[string]*Request{
		"nil request":  nil,
		"empty method": NewRequest("", struct{}{}),
		"nil payload":  NewRequest("getMe", nil),
	}
	for name, req := range tests {
		if _, err := c.Call(ctx, req); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestCallRawValidation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent")
	})
	ctx := context.Background()

	tests := map[string]*RawRequest{
		"nil request":        nil,
		"empty method":       NewRawRequest("", "text/plain", nil),
		"empty content type": NewRawRequest("getMe", "", nil),
	}
	for name, req := range tests {
		if _, err := c.CallRaw(ctx, req); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

type testMessage struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
}

func TestDoDecodesResult(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendMessage" {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"ok":true,"result":{"message_id":7,"text":"hi"}}`)
	})

	msg, err := c.Do[testMessage](context.Background(), "sendMessage", map[string]any{"text": "hi"}).Value()
	if err != nil {
		t.Fatal(err)
	}
	if msg != (testMessage{MessageID: 7, Text: "hi"}) {
		t.Errorf("msg = %+v", msg)
	}
}

func TestDoScalarResult(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true,"result":true}`)
	})

	ok, err := c.Do[bool](context.Background(), "deleteMessage", struct{}{}).Value()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("result = false")
	}
}

func TestDoAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":false,"error_code":403,"description":"Forbidden"}`)
	})

	res := c.Do[testMessage](context.Background(), "sendMessage", struct{}{})
	if res.IsOk() {
		t.Fatal("want error")
	}
	var apiErr *Error
	if !errors.As(res.Error(), &apiErr) || apiErr.Code != 403 {
		t.Errorf("err = %v, want *Error with code 403", res.Error())
	}
}

func TestDoDecodeError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true,"result":"not an object"}`)
	})

	if res := c.Do[testMessage](context.Background(), "sendMessage", struct{}{}); res.IsOk() {
		t.Fatalf("want decode error, got %v", res)
	}
}

type testSendMessage struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func (testSendMessage) Method() string { return "sendMessage" }

type testGetMe struct{}

func (*testGetMe) Method() string { return "getMe" }

func TestSendUsesRequestAsPayload(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendMessage" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"chat_id":1,"text":"hi"}` {
			t.Errorf("body = %s", body)
		}
		io.WriteString(w, `{"ok":true,"result":{"message_id":7,"text":"hi"}}`)
	})

	msg, err := c.Send[testMessage](context.Background(), testSendMessage{ChatID: 1, Text: "hi"}).Value()
	if err != nil {
		t.Fatal(err)
	}
	if msg.MessageID != 7 {
		t.Errorf("msg = %+v", msg)
	}
}

func TestSendPointerReceiverEmptyStruct(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/getMe" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{}` {
			t.Errorf("body = %s", body)
		}
		io.WriteString(w, `{"ok":true,"result":{"id":1}}`)
	})

	me, err := c.Send[map[string]int](context.Background(), &testGetMe{}).Value()
	if err != nil {
		t.Fatal(err)
	}
	if me["id"] != 1 {
		t.Errorf("me = %v", me)
	}
}
