/*
|--------------------------------------------------------------------------
| Values Over JSON
|--------------------------------------------------------------------------
|
| A render's values can travel as JSON, such as a page's variables sent
| to a tool that renders the same page elsewhere. JSON has no HTML and
| no time, so Encode writes each as a one-key object, {"$html": ...} and
| {"$time": ...}, and Decode reads them back, so markup stays markup and
| text stays text.
|
*/

package liquid

import "time"

/*
Encode turns template values into values JSON can carry.
*/
func Encode(v any) any {
	switch x := v.(type) {
	case HTML:
		return map[string]any{"$html": string(x)}
	case time.Time:
		return map[string]any{"$time": x.Format(time.RFC3339Nano)}
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = Encode(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = Encode(item)
		}
		return out
	}
	return v
}

/*
Decode reads values Encode wrote, after JSON decoding, back into
template values. Whole numbers JSON read as float64 become ints again.
*/
func Decode(v any) any {
	switch x := v.(type) {
	case float64:
		if x == float64(int(x)) {
			return int(x)
		}
		return x
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = Decode(item)
		}
		return out
	case map[string]any:
		return decodeMap(x)
	}
	return v
}

func decodeMap(x map[string]any) any {
	if v, ok := tagged(x); ok {
		return v
	}
	out := make(map[string]any, len(x))
	for k, item := range x {
		out[k] = Decode(item)
	}
	return out
}

/*
tagged reads a one-key object Encode wrote for HTML or a time.
*/
func tagged(x map[string]any) (any, bool) {
	if len(x) != 1 {
		return nil, false
	}
	if s, ok := x["$html"].(string); ok {
		return HTML(s), true
	}
	s, ok := x["$time"].(string)
	if !ok {
		return nil, false
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	return t, err == nil
}
