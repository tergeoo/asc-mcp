package mcperr

import "testing"

func TestFromResponseClassification(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantKind Kind
		wantNil  bool
	}{
		{"200 ok", 200, "", true},
		{"201 created", 201, "", true},
		{"401 auth", 401, KindAuth, false},
		{"403 auth", 403, KindAuth, false},
		{"404 not found", 404, KindNotFound, false},
		{"409 validation", 409, KindValidation, false},
		{"422 validation", 422, KindValidation, false},
		{"429 rate limit", 429, KindRateLimit, false},
		{"400 conflict", 400, KindConflict, false},
		{"500 server", 500, KindServer, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := FromResponse(tc.status, "", nil)
			if tc.wantNil {
				if e != nil {
					t.Fatalf("expected nil for %d, got %v", tc.status, e)
				}
				return
			}
			if e == nil {
				t.Fatalf("expected error for %d", tc.status)
			}
			if e.Kind != tc.wantKind {
				t.Fatalf("status %d: kind=%s, want %s", tc.status, e.Kind, tc.wantKind)
			}
		})
	}
}

func TestFromResponseParsesASCErrors(t *testing.T) {
	body := []byte(`{"errors":[{"code":"ENTITY_ERROR","status":"409","title":"Conflict","detail":"The attribute 'description' cannot be edited at this time."}]}`)
	e := FromResponse(409, "", body)
	if e == nil {
		t.Fatal("expected error")
	}
	if len(e.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(e.Details))
	}
	if e.Details[0].Code != "ENTITY_ERROR" {
		t.Fatalf("unexpected code: %s", e.Details[0].Code)
	}
	if e.Message == "" {
		t.Fatal("expected a human message")
	}
}

func TestFromResponseRetryAfter(t *testing.T) {
	e := FromResponse(429, "30", nil)
	if e.RetryAfter != "30" {
		t.Fatalf("expected RetryAfter=30, got %q", e.RetryAfter)
	}
}

func TestWrap(t *testing.T) {
	e := Wrap("list_apps", errExample{})
	if e.Kind != KindClient {
		t.Fatalf("expected client kind, got %s", e.Kind)
	}
}

type errExample struct{}

func (errExample) Error() string { return "boom" }
