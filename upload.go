package lithium

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"slices"
)

// File is a file sent along with a request by [Client.Upload] and [Client.UploadStream].
type File struct {
	// Field is the name of the form field, e.g. "photo" or "document". To reference
	// the file from an InputMedia object as "attach://name", use that name here.
	Field string
	// Name is the file name reported to the Bot API. If empty, Field is used.
	Name string
	// Reader supplies the file contents. It is read once, until EOF; closing it is
	// up to the caller.
	Reader io.Reader
}

// form is a request prepared for sending as multipart/form-data.
type form struct {
	method string
	fields []formField
	files  []File
}

// formField is a single non-file parameter of a form.
type formField struct {
	name, value string
}

// newForm validates req and files and encodes the parameters of req.
//
// req is marshaled as JSON, so its json tags apply, and must encode to an object.
// Each of its top-level keys becomes a form field: strings are sent as is, nulls
// are skipped and everything else (numbers, booleans, objects, arrays) is sent as
// JSON, as the Bot API expects.
func newForm(req Method, files []File) (*form, error) {
	if req == nil {
		return nil, errors.New("request must not be empty")
	}
	f := &form{method: req.Method(), files: files}
	if f.method == "" {
		return nil, errors.New("method must not be empty")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(data, &params); err != nil || params == nil {
		return nil, errors.New("encode payload: request must encode to a JSON object")
	}

	for _, name := range slices.Sorted(maps.Keys(params)) {
		raw := params[name]
		switch {
		case bytes.Equal(raw, []byte("null")):
			delete(params, name)
		case raw[0] == '"':
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				return nil, fmt.Errorf("encode payload: field %q: %w", name, err)
			}
			f.fields = append(f.fields, formField{name, s})
		default:
			f.fields = append(f.fields, formField{name, string(raw)})
		}
	}

	seen := make(map[string]bool, len(files))
	for i, file := range files {
		switch {
		case file.Field == "":
			return nil, fmt.Errorf("file %d: field must not be empty", i)
		case file.Reader == nil:
			return nil, fmt.Errorf("file %q: reader must not be nil", file.Field)
		case seen[file.Field]:
			return nil, fmt.Errorf("file %q: duplicate field", file.Field)
		}
		if _, ok := params[file.Field]; ok {
			return nil, fmt.Errorf("file %q: field is already set by the request", file.Field)
		}
		seen[file.Field] = true
	}
	return f, nil
}

// write writes the form to mw and closes it.
func (f *form) write(mw *multipart.Writer) error {
	for _, field := range f.fields {
		if err := mw.WriteField(field.name, field.value); err != nil {
			return fmt.Errorf("write field %q: %w", field.name, err)
		}
	}
	for _, file := range f.files {
		name := file.Name
		if name == "" {
			name = file.Field
		}
		w, err := mw.CreateFormFile(file.Field, name)
		if err != nil {
			return fmt.Errorf("write file %q: %w", file.Field, err)
		}
		if _, err := io.Copy(w, file.Reader); err != nil {
			return fmt.Errorf("write file %q: %w", file.Field, err)
		}
	}
	return mw.Close()
}
