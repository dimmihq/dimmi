package intercom

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/dimmihq/dimmi/cli/connect"
)

/*
Help walks the person through making an app in their own Intercom
Developer Hub and copying its token: Intercom has no API keys apart
from an app's.
*/
const Help template.HTML = `
<p>dimmi reads Intercom with a token from an app in your own Intercom workspace. It only reads.</p>
<ol>
  <li>In Intercom, open <b>Settings → Integrations → Developer Hub</b>.</li>
  <li>Click <b>New app</b>, name it <b>dimmi migration</b> and click <b>Create app</b>. The first time, Intercom asks you to accept its developer terms.</li>
  <li>The app opens on <b>Authentication</b>. Copy the <b>Access token</b> and paste it below.</li>
</ol>
<p class="note">When you've moved for good, delete the app in the Developer Hub and the token stops working.</p>
`

/*
Setup is Intercom's part of the connect page. Save keeps the token and
region; Saved reports the account it belongs to.
*/
func Setup(save func(region, token string) error, saved func() (string, bool)) connect.Setup {
	return connect.Setup{
		Tool: "Intercom",
		Help: Help,
		Regions: []connect.Option{
			{Value: "us", Label: "United States (app.intercom.com)"},
			{Value: "eu", Label: "Europe (app.eu.intercom.com)"},
			{Value: "au", Label: "Australia (app.au.intercom.com)"},
		},
		Check: func(ctx context.Context, region, token string) (string, error) {
			c, err := NewClient(&http.Client{Timeout: 20 * time.Second}, region, token)
			if err != nil {
				return "", err
			}

			acct, err := New(c, region, nil).Account(ctx)
			if err != nil {
				return "", err
			}

			return acct.Name, nil
		},
		Save:  save,
		Saved: saved,
	}
}
