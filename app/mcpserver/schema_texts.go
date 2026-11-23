/*
|--------------------------------------------------------------------------
| Written Schema Texts
|--------------------------------------------------------------------------
|
| What each field of the API's records means, written by hand from the
| code that fills it and kept by schema and field, and the one text for
| each request field several operations share and would explain
| differently. They go into the API document, where the reference, the
| SDKs and the tools read them.
|
*/

package mcpserver

/*
schemaTexts returns the written texts of the API's schemas, by schema
and then by field.
*/
func schemaTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range schemaTextParts() {
		mergeTexts(out, part)
	}

	return out
}
