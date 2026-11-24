/*
|--------------------------------------------------------------------------
| Written Argument Texts
|--------------------------------------------------------------------------
|
| The arguments the API document and the catalog leave without a word,
| each explained by hand from what the code behind it accepts: what the
| value is, what it may be, and what one looks like. They are kept by
| tool and argument, since the same name means different things in
| different tools. A field inside an argument is kept by its path, such
| as contact.email, or edits.find for a field of each item of a list.
| The audit fails for an argument left with only what its name and type
| say, or a field with no word at all, so a new one is written here
| before it ships.
|
*/

package mcpserver

import "maps"

/*
writtenTexts returns the written texts, by tool name without its prefix
and then by argument.
*/
func writtenTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, area := range []map[string]map[string]string{
		aiTexts(), agentStudioTexts(), ticketTexts(), otherTexts(), nestedTexts(), midNestedTexts(),
		moreNestedTexts(),
	} {
		mergeTexts(out, area)
	}

	return out
}

/*
mergeTexts adds texts to those already kept, tool by tool.
*/
func mergeTexts(into, from map[string]map[string]string) {
	for tool, texts := range from {
		if into[tool] == nil {
			into[tool] = map[string]string{}
		}

		maps.Copy(into[tool], texts)
	}
}
