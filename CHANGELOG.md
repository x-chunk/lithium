# Changelog

All notable changes to this module are documented in this file.
The module follows [Semantic Versioning](https://semver.org); while it is at v0,
a minor release may contain breaking changes.

## [v0.3.0] - 2026-10-20

### BREAKING CHANGE

All client methods now return idiomatic `(value, error)` pairs instead of
`result.Result` from `go.xchunk.org/anvil/v2/result`. The module no longer
depends on `go.xchunk.org/anvil/v2`.

| Method                      | Before                     | After                   |
|-----------------------------|----------------------------|-------------------------|
| `Client.Call`               | `result.Result[*Response]` | `(*Response, error)`    |
| `Client.CallRaw`            | `result.Result[*Response]` | `(*Response, error)`    |
| `Client.Do[Resp]`           | `result.Result[Resp]`      | `(Resp, error)`         |
| `Client.Send[Resp]`         | `result.Result[Resp]`      | `(Resp, error)`         |
| `Client.Upload[Resp]`       | `result.Result[Resp]`      | `(Resp, error)`         |
| `Client.UploadStream[Resp]` | `result.Result[Resp]`      | `(Resp, error)`         |

On error, `Call` and `CallRaw` return a nil `*Response`, and the generic methods
return the zero value of `Resp`. A Bot API error is still an `*lithium.Error`
retrievable with `errors.As`.

#### Migration

Before:

```go
msg, err := client.Send[Message](ctx, SendMessage{ChatID: 1, Text: "hi"}).Value()

err := client.Do[bool](ctx, "deleteMessage", payload).Error()

if res := client.Call(ctx, req); res.IsOk() {
	// ...
}
```

After:

```go
msg, err := client.Send[Message](ctx, SendMessage{ChatID: 1, Text: "hi"})

_, err := client.Do[bool](ctx, "deleteMessage", payload)

if resp, err := client.Call(ctx, req); err == nil {
	// ...
}
```

Chains built with `result.Result` combinators, such as `AndThen`, become plain
`if err != nil` checks.

## [v0.2.0]

### Added

- `Client.Upload` and `Client.UploadStream` for `multipart/form-data` requests with files.
- `File` describing a file part of an upload.

[v0.3.0]: https://github.com/x-chunk/lithium/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/x-chunk/lithium/compare/v0.1.1...v0.2.0
