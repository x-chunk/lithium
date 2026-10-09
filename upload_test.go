package lithium

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
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
