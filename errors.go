package gateway

import "fmt"

// APIError is an unsuccessful Telegram response with an upstream error code.
// Use errors.As to inspect it. Code is preserved without maintaining a closed
// list of Telegram errors. Response bodies and access tokens are never included.
type APIError struct {
	Method     string
	StatusCode int
	Code       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram gateway: %s: %s (HTTP %d)", e.Method, e.Code, e.StatusCode)
}

// HTTPError is a non-2xx response without a valid Telegram error envelope.
// The response body is deliberately excluded: it can contain sensitive data.
type HTTPError struct {
	Method     string
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("telegram gateway: %s: HTTP %d", e.Method, e.StatusCode)
}
