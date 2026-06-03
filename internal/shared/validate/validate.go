// Package validate enforces App Store Connect domain constraints before any
// request is sent (spec §8): field length limits, editable version states and
// allowed screenshot display types. Failing fast here yields clear errors and
// avoids predictable 409s from Apple.
package validate

import (
	"fmt"
	"strings"
)

// Field length limits (in characters) for App Store metadata.
const (
	MaxName             = 30
	MaxSubtitle         = 30
	MaxKeywords         = 100
	MaxPromotionalText  = 170
	MaxDescription      = 4000
	MaxWhatsNew         = 4000
	MaxPromoTextIAP     = 45
	MaxIAPName          = 30
	MaxIAPDescription   = 45
)

// EditableStates are the appStoreState values in which version text fields may
// be edited. Editing in any other state yields a validation error from Apple.
var EditableStates = map[string]bool{
	"PREPARE_FOR_SUBMISSION":      true,
	"DEVELOPER_REJECTED":          true,
	"REJECTED":                    true,
	"METADATA_REJECTED":           true,
	"INVALID_BINARY":              true,
	"WAITING_FOR_REVIEW":          false,
	"IN_REVIEW":                   false,
	"PENDING_DEVELOPER_RELEASE":   false,
	"READY_FOR_SALE":              false,
}

// Error is a validation failure aggregating one or more field problems.
type Error struct {
	Fields []FieldError `json:"fields"`
}

// FieldError describes a single invalid field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", f.Field, f.Message))
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// HasErrors reports whether any field errors were recorded.
func (e *Error) HasErrors() bool { return len(e.Fields) > 0 }

// Builder accumulates field validations.
type Builder struct{ err Error }

// NewBuilder returns an empty validation builder.
func NewBuilder() *Builder { return &Builder{} }

// MaxLen records an error if s exceeds max runes. Empty strings are skipped
// (treat as "not provided").
func (b *Builder) MaxLen(field, s string, max int) *Builder {
	if s == "" {
		return b
	}
	if n := len([]rune(s)); n > max {
		b.err.Fields = append(b.err.Fields, FieldError{
			Field:   field,
			Message: fmt.Sprintf("length %d exceeds maximum %d", n, max),
		})
	}
	return b
}

// Required records an error if s is empty.
func (b *Builder) Required(field, s string) *Builder {
	if strings.TrimSpace(s) == "" {
		b.err.Fields = append(b.err.Fields, FieldError{Field: field, Message: "is required"})
	}
	return b
}

// Result returns a non-nil *Error if any validations failed.
func (b *Builder) Result() *Error {
	if b.err.HasErrors() {
		return &b.err
	}
	return nil
}

// VersionStateEditable reports whether version text fields may be edited in the
// given appStoreState, and a human message when not.
func VersionStateEditable(state string) (bool, string) {
	if state == "" {
		return true, "" // unknown state — let Apple validate
	}
	editable, known := EditableStates[state]
	if !known {
		return true, "" // unknown/forward-compatible state — defer to Apple
	}
	if !editable {
		return false, fmt.Sprintf(
			"version is in state %q where text fields are not editable; "+
				"edit is only allowed in states like PREPARE_FOR_SUBMISSION or DEVELOPER_REJECTED",
			state,
		)
	}
	return true, ""
}
