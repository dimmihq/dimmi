package routing

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

/*
Schemas returns the JSON schemas of a Go type and the types it holds,
by name, the way an API document describes them. Contracts outside an
API group, such as a webhook body, are kept with it.
*/
func Schemas[T any]() (map[string]any, error) {
	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	registry.Schema(reflect.TypeFor[T](), true, "")

	raw, err := json.Marshal(registry.Map())
	if err != nil {
		return nil, fmt.Errorf("encode schemas: %w", err)
	}

	var schemas map[string]any
	if err := json.Unmarshal(raw, &schemas); err != nil {
		return nil, fmt.Errorf("decode schemas: %w", err)
	}

	return schemas, nil
}
