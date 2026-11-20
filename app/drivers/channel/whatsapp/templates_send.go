/*
|--------------------------------------------------------------------------
| Sending Templates
|--------------------------------------------------------------------------
|
| A reply sent as a template carries its choice in the reply's channel
| options: the template's name and language and the values of its
| variables. It goes to Meta as the template with those values in place,
| by position or by name as the template was written.
|
*/

package whatsapp

import (
	"encoding/json"
	"fmt"
	"strings"
)

/*
Choice is a template a reply sends: its name, language and the values
of its variables, keyed by variable key or by part and key.
*/
type Choice struct {
	Name     string            `json:"name"`
	Language string            `json:"language"`
	Values   map[string]string `json:"values,omitempty"`
}

/*
Options are the WhatsApp choices a reply carries.
*/
type Options struct {
	Template *Choice `json:"template,omitempty"`
}

/*
ParseOptions reads a reply's channel options, keyed by channel type.
*/
func ParseOptions(raw []byte) (Options, error) {
	if len(raw) == 0 {
		return Options{}, nil
	}

	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return Options{}, fmt.Errorf("parse channel options: %w", err)
	}

	var out Options
	if mine, ok := all[Type]; ok {
		if err := json.Unmarshal(mine, &out); err != nil {
			return Options{}, fmt.Errorf("parse whatsapp options: %w", err)
		}
	}

	return out, nil
}

/*
templatePayload is the template as Meta's send call takes it.
*/
func templatePayload(t Template, choice Choice) map[string]any {
	named := strings.EqualFold(t.ParameterFormat, "NAMED")

	var components []map[string]any

	for _, part := range []string{"header", "body"} {
		var params []map[string]any

		for _, v := range t.Variables() {
			if v.Part != part {
				continue
			}

			value := choice.Values[part+"."+v.Key]
			if value == "" {
				value = choice.Values[v.Key]
			}

			p := map[string]any{"type": "text", "text": value}
			if named {
				p["parameter_name"] = v.Key
			}

			params = append(params, p)
		}

		if len(params) > 0 {
			components = append(components, map[string]any{"type": part, "parameters": params})
		}
	}

	out := map[string]any{"name": t.Name, "language": map[string]string{"code": t.Language}}
	if len(components) > 0 {
		out["components"] = components
	}

	return out
}
