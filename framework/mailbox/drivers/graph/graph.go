/*
|--------------------------------------------------------------------------
| Microsoft Graph Account
|--------------------------------------------------------------------------
|
| A Microsoft mailbox read through Graph with small REST calls, since the
| SDK is far larger than these few endpoints are worth. Each folder has
| its own delta feed. Sent mail is walked first, so a reply to something
| just sent finds what it refers to. Ids are asked for in their immutable
| form, or a message would change id whenever it moved.
|
| Graph's subscriptions only wake a sync: a delta link always recovers
| whatever a lost notification missed.
|
*/

package graph

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/internal/webhook"
)

/*
Where the API lives and the limits of a call.
*/
const (
	defaultBaseURL = "https://graph.microsoft.com/v1.0"
	sentFolder     = "sentitems"
	apiTimeout     = 30 * time.Second
	maxRawBytes    = 64 << 20
	selectFields   = "id,internetMessageId"
	immutableIDs   = `IdType="ImmutableId"`
	subscription   = 6 * 24 * time.Hour
	sinceMargin    = 24 * time.Hour
)

/*
Options configure the account: the name of the folder mail comes into,
and the API address, which only tests change.
*/
type Options struct {
	ReceivedFolder string
	BaseURL        string
}

/*
Account reads Microsoft mailboxes.
*/
type Account struct {
	client *httpclient.Client
	clock  clock.Clock
	opts   Options
}

/*
New returns an account client.
*/
func New(client *httpclient.Client, source clock.Clock, opts Options) *Account {
	if opts.BaseURL == "" {
		opts.BaseURL = defaultBaseURL
	}

	return &Account{client: client, clock: source, opts: opts}
}

/*
Name returns the driver name.
*/
func (a *Account) Name() string {
	return "graph"
}

func (a *Account) api(s mailbox.Session) *httpclient.Request {
	return a.client.New().Timeout(apiTimeout).WithToken(s.Token).MaxBody(maxRawBytes).WithHeader("Prefer", immutableIDs)
}

func (a *Account) user(s mailbox.Session, path string) string {
	return a.opts.BaseURL + "/users/" + url.PathEscape(s.Mailbox) + path
}

func (a *Account) getJSON(ctx context.Context, s mailbox.Session, address string, out any) error {
	resp, err := a.api(s).Accept("application/json").Get(ctx, address)
	if err != nil {
		return fmt.Errorf("graph read: %w", err)
	}

	if err := check(resp); err != nil {
		return err
	}

	return resp.JSON(out)
}

/*
Profile returns who signed in. A session without a mailbox asks about the
signed-in account itself.
*/
func (a *Account) Profile(ctx context.Context, s mailbox.Session) (mailbox.Profile, error) {
	var me struct {
		Mail              string `json:"mail"`
		UserPrincipalName string
	}
	if err := a.getJSON(ctx, s, a.opts.BaseURL+"/me", &me); err != nil {
		return mailbox.Profile{}, err
	}

	if me.Mail != "" {
		return mailbox.Profile{Email: me.Mail}, nil
	}

	return mailbox.Profile{Email: me.UserPrincipalName}, nil
}

/*
cursor holds the next link of each folder's feed.
*/
type cursor struct {
	Received string `json:"received_delta"`
	Sent     string `json:"sent_delta"`
}

func (c cursor) bytes() []byte {
	body, _ := json.Marshal(c)

	return body
}

/*
Cursor walks each feed from now on, which yields a delta link and no
messages. A filter on the time is the documented way to skip history.
*/
func (a *Account) Cursor(ctx context.Context, s mailbox.Session) ([]byte, error) {
	from := url.Values{
		"$select": {selectFields},
		"$filter": {"receivedDateTime ge " + a.clock.Now().UTC().Format(time.RFC3339)},
	}.Encode()

	received, err := a.settle(ctx, s, a.delta(s, a.opts.ReceivedFolder, from))
	if err != nil {
		return nil, err
	}

	sent, err := a.settle(ctx, s, a.delta(s, sentFolder, from))
	if err != nil {
		return nil, err
	}

	return cursor{Received: received, Sent: sent}.bytes(), nil
}

func (a *Account) delta(s mailbox.Session, folder, query string) string {
	return a.user(s, "/mailFolders/"+folder+"/messages/delta?"+query)
}

/*
settle follows a feed to its delta link.
*/
func (a *Account) settle(ctx context.Context, s mailbox.Session, link string) (string, error) {
	for link != "" {
		page, err := a.page(ctx, s, link)
		if err != nil {
			return "", err
		}

		if page.DeltaLink != "" {
			return page.DeltaLink, nil
		}

		link = page.NextLink
	}

	return "", mailbox.ErrCursorExpired
}

/*
deltaPage is one page of a delta feed. Graph names its links with dotted
keys, so they are read by hand.
*/
type deltaPage struct {
	Value []struct {
		ID                string
		InternetMessageID string
		Removed           json.RawMessage `json:"@removed"`
	} `json:"value"`
	NextLink  string `json:"-"`
	DeltaLink string `json:"-"`
}

/*
UnmarshalJSON reads the page and its dotted link keys.
*/
func (p *deltaPage) UnmarshalJSON(data []byte) error {
	type plain deltaPage

	var links map[string]json.RawMessage
	if err := json.Unmarshal(data, &links); err != nil {
		return fmt.Errorf("decode graph delta page: %w", err)
	}

	if err := json.Unmarshal(data, (*plain)(p)); err != nil {
		return fmt.Errorf("decode graph delta page: %w", err)
	}

	for key, into := range map[string]*string{"@odata.nextLink": &p.NextLink, "@odata.deltaLink": &p.DeltaLink} {
		if link, ok := links[key]; ok {
			if err := json.Unmarshal(link, into); err != nil {
				return fmt.Errorf("decode graph delta link: %w", err)
			}
		}
	}

	return nil
}

func (a *Account) page(ctx context.Context, s mailbox.Session, link string) (deltaPage, error) {
	var page deltaPage

	err := a.getJSON(ctx, s, link, &page)

	return page, err
}

func (p deltaPage) link() string {
	if p.NextLink != "" {
		return p.NextLink
	}

	return p.DeltaLink
}

func (p deltaPage) changes(folder mailbox.Folder) []mailbox.Change {
	items := make([]mailbox.Change, 0, len(p.Value))

	for _, message := range p.Value {
		if len(message.Removed) > 0 {
			continue
		}

		items = append(items, mailbox.Change{
			ID: message.ID, Folder: folder,
			RFCMessageID: strings.Trim(message.InternetMessageID, "<> "),
		})
	}

	return items
}

/*
Changes returns one page of each feed, sent mail first. Received mail
waits until sent mail has caught up.
*/
func (a *Account) Changes(ctx context.Context, s mailbox.Session, raw []byte) (mailbox.Changes, error) {
	var at cursor
	if err := json.Unmarshal(raw, &at); err != nil || at.Received == "" || at.Sent == "" {
		return mailbox.Changes{}, mailbox.ErrCursorExpired
	}

	sent, err := a.page(ctx, s, at.Sent)
	if err != nil {
		return mailbox.Changes{}, err
	}

	at.Sent = sent.link()
	items := sent.changes(mailbox.FolderSent)

	if sent.NextLink != "" {
		return mailbox.Changes{Items: items, Cursor: at.bytes(), More: true}, nil
	}

	received, err := a.page(ctx, s, at.Received)
	if err != nil {
		return mailbox.Changes{}, err
	}

	at.Received = received.link()

	return mailbox.Changes{
		Items: append(items, received.changes(mailbox.FolderReceived)...), Cursor: at.bytes(),
		More: received.NextLink != "",
	}, nil
}

/*
Since lists recent mail of both folders, a day early so nothing falls
between, and starts new feeds.
*/
func (a *Account) Since(ctx context.Context, s mailbox.Session, since time.Time) (mailbox.Changes, error) {
	next, err := a.Cursor(ctx, s)
	if err != nil {
		return mailbox.Changes{}, err
	}

	query := url.Values{
		"$select": {selectFields},
		"$filter": {"receivedDateTime ge " + since.Add(-sinceMargin).UTC().Format(time.RFC3339)},
		"$top":    {"500"},
	}.Encode()

	items := []mailbox.Change{}

	for _, list := range []struct {
		folder string
		role   mailbox.Folder
	}{{sentFolder, mailbox.FolderSent}, {a.opts.ReceivedFolder, mailbox.FolderReceived}} {
		page, err := a.page(ctx, s, a.user(s, "/mailFolders/"+list.folder+"/messages?"+query))
		if err != nil {
			return mailbox.Changes{}, err
		}

		items = append(items, page.changes(list.role)...)
	}

	return mailbox.Changes{Items: items, Cursor: next}, nil
}

/*
Raw returns the raw message.
*/
func (a *Account) Raw(ctx context.Context, s mailbox.Session, id string) ([]byte, error) {
	resp, err := a.api(s).Get(ctx, a.user(s, "/messages/"+url.PathEscape(id)+"/$value"))
	if err != nil {
		return nil, fmt.Errorf("graph raw message: %w", err)
	}

	if err := check(resp); err != nil {
		return nil, err
	}

	return resp.Body, nil
}

/*
Verdicts reads the header Microsoft's servers wrote on top of the message.
Any header below it may be the sender's.
*/
func (a *Account) Verdicts(raw []byte) mailbox.Verdicts {
	verdicts, _ := webhook.TopAuthResults(raw, "")

	return verdicts
}

/*
Send sends the raw message. Graph answers with nothing: the id it used is
learned when sent mail is next read.
*/
func (a *Account) Send(ctx context.Context, s mailbox.Session, in mailbox.SendInput) (mailbox.Sent, error) {
	body := base64.StdEncoding.EncodeToString(in.Raw)

	resp, err := a.api(s).WithHeader("Content-Type", "text/plain").Post(ctx, a.user(s, "/sendMail"), []byte(body))
	if err != nil {
		return mailbox.Sent{}, fmt.Errorf("graph send: %w", err)
	}

	return mailbox.Sent{}, checkSend(resp)
}

/*
Revoke does nothing: Microsoft has no way to revoke one grant short of
ending every session of the account, so the tokens are simply dropped.
*/
func (a *Account) Revoke(context.Context, string) error {
	return nil
}
