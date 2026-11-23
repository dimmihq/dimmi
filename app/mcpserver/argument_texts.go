/*
|--------------------------------------------------------------------------
| Written Argument Texts
|--------------------------------------------------------------------------
|
| The arguments the API document and the catalog leave without a word,
| each explained by hand from what the code behind it accepts: what the
| value is, what it may be, and what one looks like. They are kept by
| tool and argument, since the same name means different things in
| different tools. The audit fails for an argument left with only what
| its name and type say, so a new one is written here before it ships.
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
	for _, area := range []map[string]map[string]string{aiTexts(), ticketTexts(), otherTexts()} {
		maps.Copy(out, area)
	}

	return out
}
