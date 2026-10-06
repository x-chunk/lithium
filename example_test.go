package lithium_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"go.xchunk.org/lithium"
)

type SendMessage struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func (SendMessage) Method() string { return "sendMessage" }

type Message struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
}

// fakeServer stands in for the Bot API server and always replies with body.
func fakeServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body)
	}))
}

func ExampleNew() {
	client := lithium.New("123456:TOKEN",
		lithium.WithBaseURL("http://localhost:8081"),
		lithium.WithHTTPClient(http.DefaultClient),
	)
	_ = client
}

func ExampleClient_Send() {
	srv := fakeServer(`{"ok":true,"result":{"message_id":1,"text":"hi"}}`)
	defer srv.Close()
	client := lithium.New("TOKEN", lithium.WithBaseURL(srv.URL))

	msg, err := client.Send[Message](context.Background(), SendMessage{ChatID: 1, Text: "hi"}).Value()
	if err != nil {
		panic(err)
	}
	fmt.Println(msg.MessageID, msg.Text)
	// Output: 1 hi
}

func ExampleClient_Do() {
	srv := fakeServer(`{"ok":true,"result":true}`)
	defer srv.Close()
	client := lithium.New("TOKEN", lithium.WithBaseURL(srv.URL))

	ok, err := client.Do[bool](context.Background(), "deleteMessage", map[string]any{
		"chat_id":    1,
		"message_id": 1,
	}).Value()
	if err != nil {
		panic(err)
	}
	fmt.Println(ok)
	// Output: true
}

func ExampleClient_Call() {
	srv := fakeServer(`{"ok":true,"result":{"id":1,"is_bot":true}}`)
	defer srv.Close()
	client := lithium.New("TOKEN", lithium.WithBaseURL(srv.URL))

	resp, err := client.Call(context.Background(), lithium.NewRequest("getMe", struct{}{})).Value()
	if err != nil {
		panic(err)
	}
	fmt.Println(string(resp.Bytes()))
	// Output: {"id":1,"is_bot":true}
}

func ExampleError() {
	srv := fakeServer(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":3}}`)
	defer srv.Close()
	client := lithium.New("TOKEN", lithium.WithBaseURL(srv.URL))

	err := client.Send[Message](context.Background(), SendMessage{ChatID: 1, Text: "hi"}).Error()

	var apiErr *lithium.Error
	if errors.As(err, &apiErr) {
		fmt.Println(apiErr.Code, apiErr.RetryAfter)
	}
	// Output: 429 3
}
