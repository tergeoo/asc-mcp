package store

import (
	"context"
	"testing"
)

func TestHashInputDeterministicAcrossKeyOrder(t *testing.T) {
	a := map[string]any{"locale": "en-US", "name": "App", "nested": map[string]any{"x": 1, "y": 2}}
	b := map[string]any{"nested": map[string]any{"y": 2, "x": 1}, "name": "App", "locale": "en-US"}
	if HashInput("update", a) != HashInput("update", b) {
		t.Fatal("hash should be independent of map key ordering")
	}
}

func TestHashInputDiffersByTool(t *testing.T) {
	in := map[string]any{"id": "1"}
	if HashInput("create", in) == HashInput("update", in) {
		t.Fatal("hash should incorporate the tool name")
	}
}

func TestHashInputDiffersByValue(t *testing.T) {
	if HashInput("t", map[string]any{"v": "a"}) == HashInput("t", map[string]any{"v": "b"}) {
		t.Fatal("hash should differ for different values")
	}
}

func TestNoopStore(t *testing.T) {
	n := Noop{}
	if n.Enabled() {
		t.Fatal("noop store should report disabled")
	}
	if _, found, err := n.LookupOperation(context.Background(), "h"); found || err != nil {
		t.Fatalf("noop lookup should be empty: found=%v err=%v", found, err)
	}
	if err := n.SaveOperation(context.Background(), Operation{}); err != nil {
		t.Fatalf("noop save should be no-op: %v", err)
	}
}
