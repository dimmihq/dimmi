/*
|--------------------------------------------------------------------------
| Tool Layer Calls
|--------------------------------------------------------------------------
|
| call_tool names a tool behind the layer and carries its arguments.
| Before the tool runs the layer checks that the tool is one it holds
| and that the arguments fit its schema, and answers a wrong call in a
| short sentence the model can fix.
|
*/

package toolset

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/support/actionschema"
)

/*
Call is a checked call_tool request: the tool, its arguments without
the layer's own, and the fields to keep of its result.
*/
type Call struct {
	Name      string
	Arguments json.RawMessage
	Fields    []string
}

/*
Inner is the tool a call_tool request names and the arguments it
carries for it, an empty object when it carries none.
*/
func Inner(args json.RawMessage) (string, json.RawMessage) {
	var in struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	_ = json.Unmarshal(orEmpty(args), &in)
	return in.Name, orEmpty(in.Arguments)
}

/*
Check reads a call_tool request and checks it: the tool exists, is
loaded, and its arguments match its schema. A refusal is one short
sentence the model can act on.
*/
func (s *Set) Check(args json.RawMessage) (Call, error) {
	var in struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return Call{}, s.refuse(err)
	}
	if in.Name == ReadRest {
		return Call{Name: ReadRest, Arguments: orEmpty(in.Arguments)}, nil
	}
	e, ok := s.entries[in.Name]
	if !ok {
		s.miss(in.Name)
		return Call{}, s.refuse(fmt.Errorf("%w %q; load a group to list its tools: %s", ErrUnknownTool, in.Name,
			s.groupNames()))
	}
	c := Call{Name: in.Name, Arguments: orEmpty(in.Arguments)}
	if e.Reads {
		c.Arguments, c.Fields = splitFields(c.Arguments)
	}
	expanded, err := s.Expand(c.Arguments)
	if err != nil {
		return Call{}, s.refuse(err)
	}
	c.Arguments = expanded
	if err := s.admit(e, c.Arguments); err != nil {
		return Call{}, err
	}
	s.mu.Lock()
	s.stats.Calls++
	s.stats.Used = append(s.stats.Used, in.Name)
	s.mu.Unlock()
	return c, nil
}

/*
admit checks a call's arguments. A tool the model never loaded runs
when they fit and counts as loaded; when they do not, the refusal
carries its definition, so the model can fix the call without a load.
*/
func (s *Set) admit(e *entry, args json.RawMessage) error {
	s.mu.Lock()
	loaded := s.loaded[e.Spec.Name]
	s.mu.Unlock()
	if err := s.validate(e, args); err != nil {
		if loaded {
			return s.refuse(err)
		}
		s.markLoaded(e.Spec.Name)
		return s.refuse(fmt.Errorf("%w; its definition: %s", err, mustJSON(definition{
			Name: e.Spec.Name, Description: e.Spec.Description, Input: inputOf(e.Entry),
		})))
	}
	if !loaded {
		s.mu.Lock()
		s.loaded[e.Spec.Name] = true
		s.stats.Direct++
		s.mu.Unlock()
	}
	return nil
}

/*
refuse counts a refused call_tool request.
*/
func (s *Set) refuse(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.ArgErrors++
	return err
}

/*
validate checks arguments against the tool's schema, compiling it on
first use. A schema that cannot be compiled is left unchecked, as the
handler still checks what it reads.
*/
func (s *Set) validate(e *entry, args json.RawMessage) error {
	schema := s.schemaOf(e)
	if schema == nil {
		return nil
	}
	value := map[string]any{}
	if err := json.Unmarshal(args, &value); err != nil {
		return fmt.Errorf("%w: arguments must be a JSON object", ErrArguments)
	}
	if err := schema.Check(value); err != nil {
		return fmt.Errorf("%w for %s: %s", ErrArguments, e.Spec.Name, shortReason(err))
	}
	return nil
}

/*
schemaOf is a tool's compiled schema, nil when it cannot be compiled.
*/
func (s *Set) schemaOf(e *entry) *actionschema.Schema {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.schema == nil && !e.unusable && len(e.Spec.Schema) > 0 {
		compiled, err := actionschema.CompileExact(e.Spec.Schema)
		e.schema, e.unusable = compiled, err != nil
	}
	return e.schema
}

/*
shortReason is the first line of a validation error, cut to a length a
model reads at a glance.
*/
func shortReason(err error) string {
	text := err.Error()
	if i := strings.IndexByte(text, '\n'); i > 0 {
		text = text[:i]
	}
	text = strings.TrimPrefix(text, actionschema.ErrInvalidArguments.Error()+": ")
	text = strings.TrimPrefix(text, "validating root: ")
	const most = 240
	if len(text) > most {
		text = text[:most] + "…"
	}
	return text
}
