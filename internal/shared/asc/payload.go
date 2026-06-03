package asc

// JSON:API payload builders. Attributes with empty values are omitted so that
// PATCH requests only touch the fields the caller actually supplied.

// Create builds a JSON:API creation document.
func Create(typ string, attrs map[string]any, rels map[string]any) map[string]any {
	data := map[string]any{"type": typ}
	if len(attrs) > 0 {
		data["attributes"] = pruneEmpty(attrs)
	}
	if len(rels) > 0 {
		data["relationships"] = rels
	}
	return map[string]any{"data": data}
}

// Update builds a JSON:API update document for an existing resource.
func Update(typ, id string, attrs map[string]any) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"type":       typ,
			"id":         id,
			"attributes": pruneEmpty(attrs),
		},
	}
}

// ToOne builds a to-one relationship object: {"data": {"type", "id"}}.
func ToOne(typ, id string) map[string]any {
	return map[string]any{"data": map[string]any{"type": typ, "id": id}}
}

// pruneEmpty removes nil and empty-string attribute values so PATCH bodies stay
// minimal. Booleans and numbers (including zero) are preserved intentionally.
func pruneEmpty(attrs map[string]any) map[string]any {
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		switch val := v.(type) {
		case nil:
			continue
		case string:
			if val == "" {
				continue
			}
		}
		out[k] = v
	}
	return out
}
