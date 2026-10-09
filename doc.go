// Package lithium is a minimal, low-level client for the Telegram Bot API.
//
// The package ships no data models: it only delivers calls to the Bot API and
// unwraps the common response envelope. Request and response types are defined
// by the user as plain structs with json tags.
//
// There are three levels of API, each built on top of the previous one:
//
//   - [Client.CallRaw] sends a pre-encoded body with an arbitrary content type
//     and returns the raw result as a [result.Result] of [Response].
//   - [Client.Call] encodes the payload as JSON and returns the raw result the
//     same way.
//   - [Client.Do] and [Client.Send] decode the result into a user type and return
//     it as a [result.Result].
//
// Methods that upload files take multipart/form-data instead of JSON.
// [Client.Upload] and [Client.UploadStream] encode a request struct as such a form
// along with a list of [File] values and decode the result like [Client.Send]:
//
//	msg, err := client.Upload[Message](ctx, SendPhoto{ChatID: 1},
//		lithium.File{Field: "photo", Name: "cat.jpg", Reader: f},
//	).Value()
//
// A request struct can carry its Bot API method name by implementing [Method]:
//
//	type SendMessage struct {
//		ChatID int64  `json:"chat_id"`
//		Text   string `json:"text"`
//	}
//
//	func (SendMessage) Method() string { return "sendMessage" }
//
//	msg, err := client.Send[Message](ctx, SendMessage{ChatID: 1, Text: "hi"}).Value()
//
// When the Bot API responds with ok=false, the result holds an [*Error], which can
// be retrieved with [errors.As] from the error returned by Value or Error.
package lithium
