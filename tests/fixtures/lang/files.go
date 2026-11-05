/*
|--------------------------------------------------------------------------
| Test Language
|--------------------------------------------------------------------------
|
| A pseudo language, en-XA, with a few messages written as accented
| English. Tests serve it to prove a message follows the reader's
| language without needing any real translation.
|
*/

package lang

import "embed"

/*
Files holds the messages, one directory per locale.
*/
//go:embed */*.json
var Files embed.FS
