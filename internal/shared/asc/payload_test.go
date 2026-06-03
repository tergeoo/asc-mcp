package asc

import (
	"reflect"
	"testing"
)

func TestCreatePrunesEmptyAttributes(t *testing.T) {
	doc := Create("appStoreVersions",
		map[string]any{"versionString": "1.0", "copyright": "", "usesIdfa": false},
		map[string]any{"app": ToOne("apps", "42")},
	)
	data := doc["data"].(map[string]any)
	attrs := data["attributes"].(map[string]any)
	if _, ok := attrs["copyright"]; ok {
		t.Fatal("empty string attribute should be pruned")
	}
	if _, ok := attrs["usesIdfa"]; !ok {
		t.Fatal("boolean false must be preserved")
	}
	if attrs["versionString"] != "1.0" {
		t.Fatalf("versionString = %v", attrs["versionString"])
	}
}

func TestToOneShape(t *testing.T) {
	got := ToOne("apps", "99")
	want := map[string]any{"data": map[string]any{"type": "apps", "id": "99"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToOne = %#v, want %#v", got, want)
	}
}

func TestUpdateIncludesID(t *testing.T) {
	doc := Update("appStoreVersionLocalizations", "loc-1", map[string]any{"keywords": "a,b"})
	data := doc["data"].(map[string]any)
	if data["id"] != "loc-1" || data["type"] != "appStoreVersionLocalizations" {
		t.Fatalf("unexpected data envelope: %#v", data)
	}
}

func TestResourceAttr(t *testing.T) {
	r := Resource{ID: "1", Attributes: []byte(`{"locale":"en-US","name":"App"}`)}
	var a struct {
		Locale string `json:"locale"`
		Name   string `json:"name"`
	}
	if err := r.Attr(&a); err != nil {
		t.Fatalf("attr: %v", err)
	}
	if a.Locale != "en-US" || a.Name != "App" {
		t.Fatalf("decoded = %+v", a)
	}
}
