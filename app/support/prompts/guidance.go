/*
|--------------------------------------------------------------------------
| Template Guidance
|--------------------------------------------------------------------------
|
| A root system prompt says how an agent communicates and holds none of
| the business's decisions: those are the team's, and the team writes
| them in the agent's guidance. So an agent made from a template starts
| with guidance already written, one file per template under
| resources/prompts/guidance: every decision a team in that job has to
| make, as a first draft the team reads, edits and makes its own. A
| decision the guidance leaves out is one the agent hands to a person.
|
| A file opens with a note for whoever edits it, which is never part of
| the guidance.
|
*/

package prompts

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	resprompts "github.com/dimmihq/dimmi/resources/prompts"
)

/*
TemplateGuidance is the guidance a new agent made from the template
starts with, and whether the template has any written for it.
*/
func TemplateGuidance(templateKey string) (string, bool) {
	if templateKey == "" || strings.ContainsAny(templateKey, "/\\.") {
		return "", false
	}
	raw, err := fs.ReadFile(resprompts.Guidance, "guidance/"+templateKey+".md")
	if err != nil {
		return "", false
	}
	text := strings.TrimSpace(notePattern().ReplaceAllString(string(raw), ""))
	return text + "\n", text != ""
}

/*
CheckGuidance reads every template's guidance file and fails on one
that is empty once its note is taken off.
*/
func CheckGuidance() error {
	entries, err := fs.ReadDir(resprompts.Guidance, "guidance")
	if err != nil {
		return fmt.Errorf("%w: read template guidance: %w", ErrInvalid, err)
	}
	var problems []error
	for _, e := range entries {
		key, isFile := strings.CutSuffix(e.Name(), ".md")
		if e.IsDir() || !isFile {
			continue
		}
		if _, ok := TemplateGuidance(key); !ok {
			problems = append(problems, fmt.Errorf("%w: guidance/%s is empty", ErrInvalid, e.Name()))
		}
	}
	return errors.Join(problems...)
}
