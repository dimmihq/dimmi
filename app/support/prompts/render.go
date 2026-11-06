/*
|--------------------------------------------------------------------------
| Prompt Renderer
|--------------------------------------------------------------------------
|
| A prompt file is plain text with {{name}} placeholders; the
| substitution is one pass of literal string replacement, so a prompt
| can never hold code, a loop or a filter. Placeholders with no value
| stay visible, which makes a forgotten variable obvious in the trace
| instead of silently empty.
|
*/

package prompts

import (
	"strings"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
Vars are the placeholders a prompt may name.
*/
type Vars map[string]string

/*
RenderFile loads and substitutes one file for the family.
*/
func (l *Loader) RenderFile(family enums.AIModelFamily, name string, vars Vars) (*File, error) {
	file, err := l.Get(family, name)
	if err != nil {
		return nil, err
	}

	out := file.Content
	for key, value := range vars {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}

	rendered := *file
	rendered.Content = out
	return &rendered, nil
}
