package lithium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
)

type testSendPhoto struct {
	ChatID      int64          `json:"chat_id"`
	Caption     string         `json:"caption,omitempty"`
	Silent      bool           `json:"disable_notification"`
	ReplyMarkup map[string]any `json:"reply_markup,omitempty"`
	Photo       *string        `json:"photo"`
}

func (testSendPhoto) Method() string { return "sendPhoto" }

// testPart is a decoded part of a multipart form.
type testPart struct {
	name, fileName, value string
}

// readForm decodes a multipart body of the given content type into its parts, in order.
func readForm(t *testing.T, contentType string, body io.Reader) []testPart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("content type = %q", contentType)
	}
	var parts []testPart
	mr := multipart.NewReader(body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			return parts
		}
		if err != nil {
			t.Fatal(err)
		}
		value, err := io.ReadAll(p)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, testPart{p.FormName(), p.FileName(), string(value)})
	}
}

// readRequestForm decodes the multipart body of r.
func readRequestForm(t *testing.T, r *http.Request) []testPart {
	t.Helper()
	return readForm(t, r.Header.Get("Content-Type"), r.Body)
}

func TestFormEncodesFields(t *testing.T) {
	req := testSendPhoto{
		ChatID:      -100,
		Caption:     `say "hi"`,
		ReplyMarkup: map[string]any{"remove_keyboard": true},
	}
	f, err := newForm(req, []File{
		{Field: "photo", Name: "cat.jpg", Reader: strings.NewReader("JPEG")},
		{Field: "thumb", Reader: strings.NewReader("THUMB")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.method != "sendPhoto" {
		t.Errorf("method = %q", f.method)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := f.write(mw); err != nil {
		t.Fatal(err)
	}

	got := readForm(t, mw.FormDataContentType(), &buf)
	want := []testPart{
		{name: "caption", value: `say "hi"`},
		{name: "chat_id", value: "-100"},
		{name: "disable_notification", value: "false"},
		{name: "reply_markup", value: `{"remove_keyboard":true}`},
		{name: "photo", fileName: "cat.jpg", value: "JPEG"},
		{name: "thumb", fileName: "thumb", value: "THUMB"},
	}
	if len(got) != len(want) {
		t.Fatalf("parts = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("part %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestFormValidation(t *testing.T) {
	photo := "file_id"
	file := func(field string) File { return File{Field: field, Reader: strings.NewReader("x")} }

	tests := map[string]struct {
		req   Method
		files []File
	}{
		"nil request":     {nil, nil},
		"empty method":    {testMethod{method: ""}, nil},
		"not an object":   {testMethod{method: "m", payload: []int{1}}, nil},
		"null payload":    {testMethod{method: "m"}, nil},
		"encode error":    {testMethod{method: "m", payload: make(chan int)}, nil},
		"empty field":     {testSendPhoto{}, []File{file("")}},
		"nil reader":      {testSendPhoto{}, []File{{Field: "photo"}}},
		"duplicate field": {testSendPhoto{}, []File{file("doc"), file("doc")}},
		"field collision": {testSendPhoto{}, []File{file("chat_id")}},
		"set by request":  {testSendPhoto{Photo: &photo}, []File{file("photo")}},
	}
	for name, tt := range tests {
		if _, err := newForm(tt.req, tt.files); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

// testMethod is a request whose method name and JSON encoding are set explicitly.
type testMethod struct {
	method  string
	payload any
}

func (m testMethod) Method() string { return m.method }

func (m testMethod) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.payload)
}

func TestUpload(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendPhoto" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.ContentLength <= 0 {
			t.Errorf("content length = %d, want a buffered body", r.ContentLength)
		}
		got := readRequestForm(t, r)
		want := []testPart{
			{name: "chat_id", value: "1"},
			{name: "disable_notification", value: "true"},
			{name: "photo", fileName: "cat.jpg", value: "JPEG"},
		}
		if len(got) != len(want) {
			t.Fatalf("parts = %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("part %d = %+v, want %+v", i, got[i], want[i])
			}
		}
		io.WriteString(w, `{"ok":true,"result":{"message_id":7}}`)
	})

	msg, err := c.Upload[testMessage](context.Background(), testSendPhoto{ChatID: 1, Silent: true},
		File{Field: "photo", Name: "cat.jpg", Reader: strings.NewReader("JPEG")},
	).Value()
	if err != nil {
		t.Fatal(err)
	}
	if msg.MessageID != 7 {
		t.Errorf("msg = %+v", msg)
	}
}

func TestUploadAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":false,"error_code":400,"description":"Bad Request: wrong file"}`)
	})

	err := c.Upload[testMessage](context.Background(), testSendPhoto{ChatID: 1},
		File{Field: "photo", Reader: strings.NewReader("x")},
	).Error()
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != 400 {
		t.Errorf("err = %v, want *Error with code 400", err)
	}
}

func TestUploadInvalidRequest(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent")
	})

	if res := c.Upload[testMessage](context.Background(), nil); res.IsOk() {
		t.Fatalf("want error, got %v", res)
	}
}

func TestUploadFileReadError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent")
	})

	errRead := errors.New("read failed")
	res := c.Upload[testMessage](context.Background(), testSendPhoto{ChatID: 1},
		File{Field: "photo", Reader: iotest.ErrReader(errRead)},
	)
	if !errors.Is(res.Error(), errRead) {
		t.Fatalf("err = %v, want %v", res.Error(), errRead)
	}
}

func TestUploadStream(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendPhoto" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.ContentLength != -1 {
			t.Errorf("content length = %d, want a streamed body", r.ContentLength)
		}
		got := readRequestForm(t, r)
		want := []testPart{
			{name: "caption", value: "hi"},
			{name: "chat_id", value: "1"},
			{name: "disable_notification", value: "false"},
			{name: "photo", fileName: "cat.jpg", value: "JPEG"},
		}
		if len(got) != len(want) {
			t.Fatalf("parts = %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("part %d = %+v, want %+v", i, got[i], want[i])
			}
		}
		io.WriteString(w, `{"ok":true,"result":{"message_id":7}}`)
	})

	msg, err := c.UploadStream[testMessage](context.Background(), testSendPhoto{ChatID: 1, Caption: "hi"},
		File{Field: "photo", Name: "cat.jpg", Reader: strings.NewReader("JPEG")},
	).Value()
	if err != nil {
		t.Fatal(err)
	}
	if msg.MessageID != 7 {
		t.Errorf("msg = %+v", msg)
	}
}

func TestUploadStreamInvalidRequest(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent")
	})

	res := c.UploadStream[testMessage](context.Background(), testSendPhoto{},
		File{Field: "photo"},
	)
	if res.IsOk() {
		t.Fatalf("want error, got %v", res)
	}
}

func TestUploadStreamFileReadError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		io.WriteString(w, `{"ok":true,"result":{}}`)
	})

	errRead := errors.New("read failed")
	res := c.UploadStream[testMessage](context.Background(), testSendPhoto{ChatID: 1},
		File{Field: "photo", Reader: iotest.ErrReader(errRead)},
	)
	if !errors.Is(res.Error(), errRead) {
		t.Fatalf("err = %v, want %v", res.Error(), errRead)
	}
}

// endlessReader yields data forever and reports reads made after done is set.
type endlessReader struct {
	t    *testing.T
	done atomic.Bool
}

func (r *endlessReader) Read(p []byte) (int, error) {
	if r.done.Load() {
		r.t.Error("file read after UploadStream returned")
	}
	clear(p)
	return len(p), nil
}

func TestUploadStreamStopsReadingOnEarlyResponse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, `{"ok":false,"error_code":413,"description":"Request Entity Too Large"}`)
	})

	file := &endlessReader{t: t}
	res := c.UploadStream[testMessage](context.Background(), testSendPhoto{ChatID: 1},
		File{Field: "photo", Reader: file},
	)
	file.done.Store(true)
	if res.IsOk() {
		t.Fatalf("want error, got %v", res)
	}
}

func TestUploadStreamContextCanceled(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent")
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	file := &endlessReader{t: t}
	res := c.UploadStream[testMessage](ctx, testSendPhoto{ChatID: 1},
		File{Field: "photo", Reader: file},
	)
	file.done.Store(true)
	if !errors.Is(res.Error(), context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", res.Error())
	}
}
