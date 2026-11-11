/*
|--------------------------------------------------------------------------
| Knowledge Source Fixtures
|--------------------------------------------------------------------------
|
| The answers of the services knowledge sources read, recorded once from
| dummy accounts filled with made-up help articles, so the drivers'
| contract tests replay real API shapes without reaching the network.
| They hold no key or real customer content.
|
*/

package sources

import (
	"embed"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
)

/*
Files holds the fixtures, one recorded session per file.
*/
//go:embed *.json
var Files embed.FS

/*
Exchanges reads the recorded session of the named file.
*/
func Exchanges(name string) ([]httpclienttest.Exchange, error) {
	data, err := Files.ReadFile(name)
	if err != nil {
		return nil, err
	}

	var out []httpclienttest.Exchange
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	return out, nil
}
