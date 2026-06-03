// Package mcperr normalizes App Store Connect API errors into a structured,
// agent-friendly form. Tools never swallow errors silently; they surface a
// typed Error that classifies the failure (auth, validation, rate limit,
// not-found, conflict) and carries Apple's own error detail.
package mcperr

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Kind classifies a failure so the agent can decide how to react.
type Kind string

const (
	KindAuth       Kind = "auth"        // 401/403
	KindValidation Kind = "validation"  // 409 / 422 — field not editable in state
	KindNotFound   Kind = "not_found"   // 404
	KindRateLimit  Kind = "rate_limit"  // 429
	KindConflict   Kind = "conflict"    // generic 4xx conflict
	KindServer     Kind = "server"      // 5xx
	KindUnknown    Kind = "unknown"     // anything else
	KindClient     Kind = "client"      // local/transport error
)

// Error is a normalized error returned to the agent.
type Error struct {
	Kind       Kind          `json:"kind"`
	StatusCode int           `json:"statusCode"`
	Message    string        `json:"message"`
	RetryAfter string        `json:"retryAfter,omitempty"`
	Details    []ASCErrDetail `json:"details,omitempty"`
}

// ASCErrDetail mirrors a single entry from Apple's errors[] array.
type ASCErrDetail struct {
	Code   string `json:"code,omitempty"`
	Title  string `json:"title,omitempty"`
	Detail string `json:"detail,omitempty"`
	Status string `json:"status,omitempty"`
}

func (e *Error) Error() string {
	if len(e.Details) > 0 {
		return fmt.Sprintf("ASC %s error (%d): %s", e.Kind, e.StatusCode, e.Details[0].Detail)
	}
	return fmt.Sprintf("ASC %s error (%d): %s", e.Kind, e.StatusCode, e.Message)
}

// ascErrorBody is the JSON:API error envelope Apple returns.
type ascErrorBody struct {
	Errors []struct {
		Code   string `json:"code"`
		Status string `json:"status"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

// FromResponse builds a normalized Error from an ASC HTTP status + raw body.
// Returns nil for 2xx responses.
func FromResponse(statusCode int, retryAfter string, body []byte) *Error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	e := &Error{
		Kind:       classify(statusCode),
		StatusCode: statusCode,
		RetryAfter: retryAfter,
	}

	var parsed ascErrorBody
	if len(body) > 0 && json.Unmarshal(body, &parsed) == nil && len(parsed.Errors) > 0 {
		for _, d := range parsed.Errors {
			e.Details = append(e.Details, ASCErrDetail{
				Code: d.Code, Title: d.Title, Detail: d.Detail, Status: d.Status,
			})
		}
		e.Message = humanMessage(e.Kind, parsed.Errors[0].Detail)
	} else {
		e.Message = humanMessage(e.Kind, string(body))
	}
	return e
}

// Wrap turns a transport/local error into a normalized client Error.
func Wrap(op string, err error) *Error {
	return &Error{
		Kind:    KindClient,
		Message: fmt.Sprintf("%s: %v", op, err),
	}
}

func classify(status int) Kind {
	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return KindAuth
	case status == http.StatusNotFound:
		return KindNotFound
	case status == http.StatusTooManyRequests:
		return KindRateLimit
	case status == http.StatusConflict, status == http.StatusUnprocessableEntity:
		return KindValidation
	case status >= 500:
		return KindServer
	case status >= 400:
		return KindConflict
	default:
		return KindUnknown
	}
}

func humanMessage(kind Kind, detail string) string {
	if detail == "" {
		detail = "no detail provided by App Store Connect"
	}
	switch kind {
	case KindAuth:
		return "authentication/authorization failed: " + detail
	case KindValidation:
		return "validation failed (field may not be editable in the current version state): " + detail
	case KindRateLimit:
		return "rate limited by App Store Connect, retry with backoff: " + detail
	case KindNotFound:
		return "resource not found: " + detail
	default:
		return detail
	}
}
