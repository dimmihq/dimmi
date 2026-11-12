/*
Package llms carries what the server hands coding assistants: llms.txt,
how to add the messenger to an app and call the API, embedded so a
self-hosted binary serves it too.
*/
package llms

import _ "embed"

/*
Text is llms.txt.
*/
//go:embed llms.txt
var Text []byte
