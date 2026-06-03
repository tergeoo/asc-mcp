#!/usr/bin/env python3
"""Prune the App Store Connect OpenAPI spec down to the operations the MCP
server actually calls, keeping every component reachable via $ref.

The full Apple spec is ~7 MB / ~900 paths / ~3000 schemas. Generating a client
from all of it produces an unusable amount of Go. We keep only the operationIds
listed in scripts/include_ops.txt plus their transitive $ref closure.

Usage:
    python3 scripts/prune_spec.py api/openapi.json scripts/include_ops.txt api/openapi.pruned.json
"""
import json
import sys
from typing import Any


def load(path: str) -> Any:
    with open(path, encoding="utf-8") as fh:
        return json.load(fh)


def strip_enums(node: Any) -> int:
    """Remove every `enum` constraint from the spec in place.

    Apple's spec defines both top-level enum schemas (e.g. `InAppPurchaseType`)
    and inline enum properties of matching names; oapi-codegen derives colliding
    Go type names from `<Schema><Property>` and aborts. Inline enums also break
    when two properties share an enum. Since this is an MCP pass-through and we
    enforce domain constraints in our own validation layer (spec §8), we drop
    the `enum` keyword entirely: top-level enums become `type Foo = string`,
    inline enum properties become plain string fields named after their JSON key.
    Returns the number of `enum` keys removed.
    """
    removed = 0
    if isinstance(node, dict):
        if "enum" in node:
            del node["enum"]
            removed += 1
        for value in node.values():
            removed += strip_enums(value)
    elif isinstance(node, list):
        for item in node:
            removed += strip_enums(item)
    return removed


def collect_refs(node: Any, acc: set[str]) -> None:
    """Walk an arbitrary JSON node and record every local $ref pointer."""
    if isinstance(node, dict):
        ref = node.get("$ref")
        if isinstance(ref, str) and ref.startswith("#/"):
            acc.add(ref)
        for value in node.values():
            collect_refs(value, acc)
    elif isinstance(node, list):
        for item in node:
            collect_refs(item, acc)


def resolve_pointer(spec: dict, ref: str) -> Any:
    parts = ref.lstrip("#/").split("/")
    cur: Any = spec
    for part in parts:
        part = part.replace("~1", "/").replace("~0", "~")
        cur = cur[part]
    return cur


def main() -> int:
    spec_path, ops_path, out_path = sys.argv[1], sys.argv[2], sys.argv[3]
    spec = load(spec_path)
    stripped = strip_enums(spec)
    wanted_ops = {
        line.strip()
        for line in open(ops_path, encoding="utf-8")
        if line.strip()
    }

    methods = {"get", "put", "post", "delete", "patch", "options", "head", "trace"}
    kept_paths: dict[str, dict] = {}
    seed_nodes: list[Any] = []

    for path, item in spec.get("paths", {}).items():
        kept_ops = {}
        shared = {k: v for k, v in item.items() if k not in methods}
        for method, op in item.items():
            if method in methods and isinstance(op, dict):
                if op.get("operationId") in wanted_ops:
                    kept_ops[method] = op
        if kept_ops:
            new_item = dict(shared)
            new_item.update(kept_ops)
            kept_paths[path] = new_item
            seed_nodes.append(new_item)

    found_ops = {
        op.get("operationId")
        for item in kept_paths.values()
        for m, op in item.items()
        if m in methods
    }
    missing = wanted_ops - found_ops
    if missing:
        print(f"ERROR: operationIds not found in spec: {sorted(missing)}", file=sys.stderr)
        return 1

    # Transitive closure over $ref.
    refs: set[str] = set()
    for node in seed_nodes:
        collect_refs(node, refs)

    resolved: set[str] = set()
    while refs - resolved:
        ref = (refs - resolved).pop()
        resolved.add(ref)
        try:
            target = resolve_pointer(spec, ref)
        except (KeyError, TypeError):
            print(f"WARN: dangling ref {ref}", file=sys.stderr)
            continue
        collect_refs(target, refs)

    # Rebuild components keeping only reached members.
    new_components: dict[str, dict] = {}
    for ref in sorted(resolved):
        parts = ref.lstrip("#/").split("/")
        if parts[0] != "components" or len(parts) < 3:
            continue
        section, name = parts[1], parts[2]
        new_components.setdefault(section, {})
        new_components[section][name] = resolve_pointer(spec, ref)

    pruned = {
        "openapi": spec.get("openapi", "3.0.1"),
        "info": spec.get("info", {}),
        "servers": spec.get("servers", []),
        "paths": kept_paths,
        "components": new_components,
    }
    if "tags" in spec:
        pruned["tags"] = spec["tags"]

    with open(out_path, "w", encoding="utf-8") as fh:
        json.dump(pruned, fh, indent=1)

    print(
        f"pruned: {len(kept_paths)} paths, "
        f"{sum(len(v) for v in new_components.values())} components, "
        f"{stripped} enum constraints stripped -> {out_path}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
