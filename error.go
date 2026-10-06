package lithium

import "fmt"

// Error is returned when the Bot API responds with ok=false.
type Error struct {
	// Code is the error_code of the response, usually mirroring the HTTP status.
	Code int
	// Description is a human-readable explanation of the error.
	Description string
	// RetryAfter is the number of seconds to wait before repeating the request (flood control).
	RetryAfter int
	// MigrateToChatID is the new identifier of a group migrated to a supergroup.
	MigrateToChatID int64
}

func (e *Error) Error() string {
	return fmt.Sprintf("telegram: %d %s", e.Code, e.Description)
}
