package internal

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestTransport(t *testing.T, h http.HandlerFunc) *Transport {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Transport{HTTPClient: srv.Client(), BaseURL: srv.URL, Token: "TOKEN"}
}

func TestURL(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{"", "https://api.telegram.org/botT/getMe"},
		{"http://localhost:8081", "http://localhost:8081/botT/getMe"},
		{"http://localhost:8081/", "http://localhost:8081/botT/getMe"},
	}
	for _, tt := range tests {
		tr := Transport{BaseURL: tt.base, Token: "T"}
		if got := tr.url("getMe"); got != tt.want {
			t.Errorf("url(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}

func TestDoSendsPost(t *testing.T) {
	tr := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "text/plain" {
			t.Errorf("content type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "body" {
			t.Errorf("body = %s", body)
		}
		io.WriteString(w, `{"ok":true,"result":1}`)
	})

	env, err := tr.Do(context.Background(), "m", "text/plain", []byte("body"))
	if err != nil {
		t.Fatal(err)
	}
	if !env.Ok || string(env.Result) != "1" {
		t.Errorf("env = %+v", env)
	}
}

func TestDoReaderStreamsBody(t *testing.T) {
	tr := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "text/plain" {
			t.Errorf("content type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "streamed" {
			t.Errorf("body = %s", body)
		}
		io.WriteString(w, `{"ok":true,"result":1}`)
	})

	pr, pw := io.Pipe()
	go func() {
		io.WriteString(pw, "stream")
		io.WriteString(pw, "ed")
		pw.Close()
	}()

	env, err := tr.DoReader(context.Background(), "m", "text/plain", pr)
	if err != nil {
		t.Fatal(err)
	}
	if !env.Ok || string(env.Result) != "1" {
		t.Errorf("env = %+v", env)
	}
}

func TestDoReaderBodyError(t *testing.T) {
	tr := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		io.WriteString(w, `{"ok":true,"result":1}`)
	})

	errBody := errors.New("body failed")
	pr, pw := io.Pipe()
	pw.CloseWithError(errBody)

	if _, err := tr.DoReader(context.Background(), "m", "text/plain", pr); !errors.Is(err, errBody) {
		t.Fatalf("err = %v, want %v", err, errBody)
	}
}

func TestDoReturnsNotOkEnvelope(t *testing.T) {
	tr := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"ok":false,"error_code":400,"description":"Bad Request","parameters":{"migrate_to_chat_id":-100}}`)
	})

	env, err := tr.Do(context.Background(), "m", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	if env.Ok || env.ErrorCode != 400 || env.Description != "Bad Request" {
		t.Errorf("env = %+v", env)
	}
	if env.Parameters == nil || env.Parameters.MigrateToChatID != -100 {
		t.Errorf("parameters = %+v", env.Parameters)
	}
}

func TestDoInvalidJSON(t *testing.T) {
	tr := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "<html>bad gateway</html>")
	})

	if _, err := tr.Do(context.Background(), "m", "application/json", nil); err == nil {
		t.Fatal("want error")
	}
}

func TestDoContextCanceled(t *testing.T) {
	tr := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent")
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := tr.Do(ctx, "m", "application/json", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestDoJSONEncodeError(t *testing.T) {
	tr := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent")
	})

	if _, err := tr.DoJSON(context.Background(), "m", make(chan int)); err == nil {
		t.Fatal("want error")
	}
}

func TestDoJSONEncodesPayload(t *testing.T) {
	tr := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"a":1}` {
			t.Errorf("body = %s", body)
		}
		io.WriteString(w, `{"ok":true,"result":true}`)
	})

	if _, err := tr.DoJSON(context.Background(), "m", map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
}
