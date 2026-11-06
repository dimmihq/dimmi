/*
|--------------------------------------------------------------------------
| Prompt Renderer
|--------------------------------------------------------------------------
|
| A prompt file is plain text with {{name}} placeholders; substitution
| is one pass of literal replacement, so a prompt can never hold code,
| a loop or a filter, and a value can never inject another placeholder.
| The boot check already proved every placeholder is one the pipeline
| fills; a value the caller leaves out renders empty.
|
*/

package prompts

import (
	"github.com/dimmihq/dimmi/app/enums"
)

/*
Vars are the placeholder values one render fills.
*/
type Vars map[string]string

/*
RenderFile renders one file of the default version for the family.
*/
func (l *Loader) RenderFile(family enums.AIModelFamily, name string, vars Vars) (*File, error) {
	return l.Render(l.version, family, name, vars)
}

/*
Render loads and substitutes one file of a version for the family.
*/
func (l *Loader) Render(version string, family enums.AIModelFamily, name string, vars Vars) (*File, error) {
	file, err := l.GetVersion(version, family, name)
	if err != nil {
		return nil, err
	}

	rendered := *file
	rendered.Content = placeholderPattern().ReplaceAllStringFunc(file.Content, func(m string) string {
		parts := placeholderPattern().FindStringSubmatch(m)
		return vars[parts[1]]
	})
	return &rendered, nil
}
