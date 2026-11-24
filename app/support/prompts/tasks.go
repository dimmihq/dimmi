/*
|--------------------------------------------------------------------------
| Task Prompts
|--------------------------------------------------------------------------
|
| Every job a model does besides answering as an agent reads its prompt
| from a file of its own under resources/prompts/tasks/: judging a
| conversation, summarizing it, sorting a message, playing a simulated
| customer. No prompt is written inline in code, so each one can be
| read, reviewed and improved as the piece of writing it is.
|
| A task file is plain markdown and is sent as written, with its
| {{placeholders}} filled. A file under tasks/judgments/ is a question
| for the decision model and has four parts, each under its own
| heading: "# State" (what the model is told it is looking at),
| "# Question", "# True when" and "# False when". A leading HTML
| comment in any file is a note for the people who edit it and is never
| sent to a model.
|
| CheckTasks reads every file when the application boots, so a missing
| part stops the boot instead of surfacing mid-conversation.
|
*/

package prompts

import (
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	resprompts "github.com/dimmihq/dimmi/resources/prompts"
)

/*
Question is one question for the decision model: what it is looking
at, what is asked, and what makes the answer true and false.
*/
type Question struct {
	State string
	Ask   string
	True  string
	False string
}

/*
judgmentParts are the headings a judgment file carries. State is the
only one a file may leave out, when the code writes the state itself.
*/
func judgmentParts() []string {
	return []string{"State", "Question", "True when", "False when"}
}

/*
readTask reads one task file, without the editors' note that opens it.
The tree is compiled in, so reading it is reading memory.
*/
func readTask(name string) string {
	data, err := fs.ReadFile(resprompts.Tasks, "tasks/"+name+".md")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(notePattern().ReplaceAllString(string(data), ""))
}

/*
notePattern matches the editors' note that opens a file.
*/
func notePattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)\A\s*<!--.*?-->`)
}

/*
Task returns a task's prompt with its placeholders filled. Name is the
file's path under tasks/ without its extension, such as
"simulation/customer". A task that does not exist is a mistake in the
code that asks for it, which the tests catch.
*/
func Task(name string, vars Vars) string {
	return fill(readTask(name), vars)
}

/*
Judgment returns a question for the decision model with its
placeholders filled. Name is the file's name under tasks/judgments/.
*/
func Judgment(name string, vars Vars) Question {
	parts := split(readTask("judgments/" + name))
	return Question{
		State: fill(parts["State"], vars), Ask: fill(parts["Question"], vars),
		True: fill(parts["True when"], vars), False: fill(parts["False when"], vars),
	}
}

/*
CheckTasks reads every task file and reports the first that is empty,
or is a judgment missing one of its parts.
*/
func CheckTasks() error {
	names := TaskNames()
	if len(names) == 0 {
		return fmt.Errorf("%w: no task prompts are compiled in", ErrInvalid)
	}
	for _, name := range names {
		text := readTask(name)
		if text == "" {
			return fmt.Errorf("%w: tasks/%s.md is empty", ErrInvalid, name)
		}
		if !strings.HasPrefix(name, "judgments/") {
			continue
		}
		parts := split(text)
		for _, part := range judgmentParts() {
			if part != "State" && parts[part] == "" {
				return fmt.Errorf("%w: tasks/%s.md has no \"# %s\" part", ErrInvalid, name, part)
			}
		}
	}
	return nil
}

/*
TaskNames lists every task file, sorted.
*/
func TaskNames() []string {
	var out []string
	_ = fs.WalkDir(resprompts.Tasks, "tasks", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") {
			out = append(out, strings.TrimSuffix(strings.TrimPrefix(path, "tasks/"), ".md"))
		}
		return err
	})
	slices.Sort(out)
	return out
}

/*
split reads a judgment file's parts by their headings.
*/
func split(text string) map[string]string {
	out := map[string]string{}
	current := ""
	var body []string
	flush := func() {
		if current != "" {
			out[current] = strings.TrimSpace(strings.Join(body, "\n"))
		}
		body = nil
	}
	for line := range strings.SplitSeq(text, "\n") {
		if heading, ok := strings.CutPrefix(line, "# "); ok {
			flush()
			current = strings.TrimSpace(heading)
			continue
		}
		body = append(body, line)
	}
	flush()
	return out
}

/*
fill substitutes a text's placeholders.
*/
func fill(text string, vars Vars) string {
	return placeholderPattern().ReplaceAllStringFunc(text, func(m string) string {
		return vars[placeholderPattern().FindStringSubmatch(m)[1]]
	})
}
