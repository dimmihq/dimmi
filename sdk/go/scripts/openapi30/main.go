/*
Command openapi30 rewrites dimmi's OpenAPI 3.1 document into the 3.0
dialect oapi-codegen reads. dimmi's spec uses only two 3.1 features: a
type list with "null", which becomes the type plus nullable: true, and
contentMediaType, which 3.0 has no word for and is dropped.

	go run ./scripts/openapi30 < api_v1.openapi.json > api/openapi30.json
*/
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		fail(fmt.Errorf("read spec: %w", err))
	}
	doc["openapi"] = "3.0.3"
	delete(doc, "jsonSchemaDialect")
	delete(doc, "webhooks")
	walk(doc)
	out := json.NewEncoder(os.Stdout)
	out.SetIndent("", "  ")
	if err := out.Encode(doc); err != nil {
		fail(err)
	}
}

func walk(node any) {
	switch v := node.(type) {
	case map[string]any:
		if types, ok := v["type"].([]any); ok {
			rewriteTypes(v, types)
		}
		delete(v, "contentMediaType")
		for _, child := range v {
			walk(child)
		}
	case []any:
		for _, child := range v {
			walk(child)
		}
	}
}

func rewriteTypes(schema map[string]any, types []any) {
	var kept []any
	for _, t := range types {
		if t == "null" {
			schema["nullable"] = true
			continue
		}
		kept = append(kept, t)
	}
	switch len(kept) {
	case 0:
		delete(schema, "type")
	case 1:
		schema["type"] = kept[0]
	default:
		delete(schema, "type")
		var alts []any
		for _, t := range kept {
			alts = append(alts, map[string]any{"type": t})
		}
		schema["anyOf"] = alts
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "openapi30:", err)
	os.Exit(1)
}
