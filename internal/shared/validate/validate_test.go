package validate

import "testing"

func TestBuilderMaxLen(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		max     int
		wantErr bool
	}{
		{"empty is skipped", "", 5, false},
		{"under limit", "abc", 5, false},
		{"at limit", "abcde", 5, false},
		{"over limit", "abcdef", 5, true},
		{"multibyte counted by rune", "ёёё", 2, true},
		{"multibyte at limit", "ёё", 2, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewBuilder().MaxLen("field", tc.value, tc.max).Result()
			if (err != nil) != tc.wantErr {
				t.Fatalf("MaxLen(%q,%d) err=%v, wantErr=%v", tc.value, tc.max, err, tc.wantErr)
			}
		})
	}
}

func TestBuilderRequired(t *testing.T) {
	if err := NewBuilder().Required("f", "").Result(); err == nil {
		t.Fatal("expected error for empty required field")
	}
	if err := NewBuilder().Required("f", "  ").Result(); err == nil {
		t.Fatal("expected error for whitespace-only required field")
	}
	if err := NewBuilder().Required("f", "x").Result(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuilderAggregatesFields(t *testing.T) {
	err := NewBuilder().
		Required("name", "").
		MaxLen("subtitle", "way too long subtitle here", 5).
		Result()
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	if len(err.Fields) != 2 {
		t.Fatalf("expected 2 field errors, got %d (%s)", len(err.Fields), err.Error())
	}
}

func TestVersionStateEditable(t *testing.T) {
	tests := []struct {
		state       string
		wantEditable bool
	}{
		{"PREPARE_FOR_SUBMISSION", true},
		{"DEVELOPER_REJECTED", true},
		{"READY_FOR_SALE", false},
		{"IN_REVIEW", false},
		{"", true},                 // unknown — defer to Apple
		{"SOME_FUTURE_STATE", true}, // forward-compatible
	}
	for _, tc := range tests {
		t.Run(tc.state, func(t *testing.T) {
			editable, msg := VersionStateEditable(tc.state)
			if editable != tc.wantEditable {
				t.Fatalf("state %q editable=%v, want %v", tc.state, editable, tc.wantEditable)
			}
			if !editable && msg == "" {
				t.Fatal("expected a non-empty message when not editable")
			}
		})
	}
}
