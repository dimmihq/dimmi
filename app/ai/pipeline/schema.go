/*
|--------------------------------------------------------------------------
| Tool Schemas
|--------------------------------------------------------------------------
|
| Some tools take a choice from a list that is only known at run time,
| such as the workspace's procedures. Their schemas are encoded as whole
| values, never pieced together from strings, so a name holding a quote
| can never change the schema's shape.
|
*/

package pipeline

import "encoding/json"

/*
enumProperty is a string argument limited to the listed values.
*/
func enumProperty(values []string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

/*
textProperty is a free string argument with a description.
*/
func textProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

/*
objectSchema is a closed object schema of the properties, each listed as
required. It holds only strings and string lists, so it always encodes.
*/
func objectSchema(properties map[string]any, required ...string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"type": "object", "properties": properties, "required": required, "additionalProperties": false,
	})

	return raw
}
