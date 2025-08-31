package events

import (
	"net/http"
	"time"
)

// HTTPStart is emitted when an HTTP request is received.
// Context carries the request context.
type HTTPStart struct {
	Request *http.Request
}

// HTTPFinish is emitted after the handler completes.
type HTTPFinish struct {
	Request  *http.Request
	Status   int
	Duration time.Duration
}
