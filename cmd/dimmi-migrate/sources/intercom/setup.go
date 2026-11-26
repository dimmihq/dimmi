package intercom

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/connect"
)

/*
Help walks the person through making a private app with read
permissions in their own Intercom Developer Hub.
*/
const Help template.HTML = `
<p>dimmi-migrate reads Intercom with a token from a private app in your own workspace. It only reads.</p>
<ol>
  <li>In Intercom, open <b>Settings → Integrations → Developer Hub</b>, or go to <code>app.intercom.com/a/apps/_/developer-hub</code>.</li>
  <li>Click <b>New app</b>. Name it <b>dimmi migration</b>, pick your workspace, and create it.</li>
  <li>Open <b>Authentication</b> in the app and click <b>Edit</b>. Tick these read permissions:
    <ul>
      <li>Read admins</li>
      <li>Read and list users and companies, Read one company</li>
      <li>Read conversations</li>
      <li>Read tickets</li>
      <li>Read tags</li>
      <li>Read data attributes</li>
      <li>Read and list articles, Read content</li>
      <li>Read workflows, if you'll bring workflows</li>
    </ul>
    Leave every write permission off. Save.</li>
  <li>Still on <b>Authentication</b>, copy the <b>Access token</b> and paste it below.</li>
</ol>
<p class="note">Once the move is done, delete the app in the Developer Hub and the token stops working.</p>
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
