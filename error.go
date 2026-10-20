package synthient

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIError is returned when a request fails with a non-success HTTP status.
// Title, Detail, Errors, and RequestID hold untrusted server data that may be
// sensitive. Use errors.AsType to inspect it:
//
//	apiErr, ok := errors.AsType[*synthient.APIError](err)
//	if ok && apiErr.StatusCode == http.StatusTooManyRequests {
//		time.Sleep(apiErr.RetryAfter)
//	}
type APIError struct {
	StatusCode int
	// Type is a URI identifying the problem type, for example
	// "https://docs.synthient.com/errors/unauthorized".
	Type string
	// Title is a short summary of the problem type.
	Title string
	// Detail explains this occurrence of the problem. For a plain-text error,
	// such as the edge's 401 for a malformed key, it is the response body.
	Detail string
	// Errors maps request fields to validation messages.
	Errors map[string][]string
	// RequestID identifies the request when contacting Synthient support.
	RequestID string
	// RetryAfter is how long the server asked the client to wait before
	// retrying, or zero if it did not say.
	RetryAfter time.Duration
	cause      error
}

// Error returns only the HTTP status code and its standard text, so it is
// safe to log.
func (e *APIError) Error() string {
	return strings.TrimSpace("synthient: " + strconv.Itoa(e.StatusCode) + " " + http.StatusText(e.StatusCode))
}

// Unwrap returns the error from reading the response body, if any, so
// errors.Is can detect cancellation and deadlines.
func (e *APIError) Unwrap() error {
	return e.cause
}

type problem struct {
	Type     string          `json:"type"`
	Title    string          `json:"title"`
	Status   int             `json:"status"`
	Detail   string          `json:"detail"`
	Instance string          `json:"instance"`
	Errors   json.RawMessage `json:"errors"`
}

func newAPIError(resp *http.Response) *APIError {
	e := &APIError{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-ID"),
		RetryAfter: parseRetryAfter(resp.Header),
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorSize))
	if err != nil {
		e.cause = err
		return e
	}
	var p problem
	err = json.Unmarshal(body, &p)
	if err != nil {
		mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if mediaType == "text/plain" {
			e.Detail = strings.TrimSpace(string(body))
		}
		return e
	}
	e.Type = p.Type
	e.Title = p.Title
	e.Detail = p.Detail
	if e.RequestID == "" {
		e.RequestID = p.Instance
	}
	if len(p.Errors) > 0 {
		_ = json.Unmarshal(p.Errors, &e.Errors)
	}
	return e
}
