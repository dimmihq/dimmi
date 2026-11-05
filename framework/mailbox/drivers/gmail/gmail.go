/*
|--------------------------------------------------------------------------
| Gmail Account
|--------------------------------------------------------------------------
|
| A Gmail mailbox read through the Gmail API with small REST calls, since
| the official SDK would add far more to the binary than these few
| endpoints are worth. The history feed says what is new, each new message
| is read for its labels and then fetched whole, and the mailbox is never
| changed: the scopes allow reading and sending only.
|
| Gmail's push only wakes a sync. The cursor is the truth, and Google warns
| that notifications can be late or dropped.
|
*/

package gmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/internal/webhook"
)

/*
Where the API lives and how long a call may take.
*/
const (
	defaultBaseURL   = "https://gmail.googleapis.com"
	defaultRevokeURL = "https://oauth2.googleapis.com/revoke"
	apiTimeout       = 30 * time.Second
	maxRawBytes      = 64 << 20
	sentLabel        = "SENT"
	authServer       = "mx.google.com"
	pageSize         = "100"
	listDayMargin    = 24 * time.Hour
)

/*
skipLabels are mail never read, whichever other label it carries.
*/
func skipLabels() []string {
	return []string{"SPAM", "TRASH", "DRAFT", "CHATS"}
}

/*
Options configure the account: the label of mail that came in, and the
API addresses, which only tests change.
*/
type Options struct {
	ReceivedLabel string
	BaseURL       string
	RevokeURL     string
}

/*
Account reads Gmail mailboxes.
*/
type Account struct {
	client *httpclient.Client
	opts   Options
}

/*
New returns an account client.
*/
func New(client *httpclient.Client, opts Options) *Account {
	if opts.BaseURL == "" {
		opts.BaseURL = defaultBaseURL
	}

	if opts.RevokeURL == "" {
		opts.RevokeURL = defaultRevokeURL
	}

	return &Account{client: client, opts: opts}
}

/*
Name returns the driver name.
*/
func (a *Account) Name() string {
	return "gmail"
}

func (a *Account) api(s mailbox.Session) *httpclient.Request {
	return a.client.New().Timeout(apiTimeout).WithToken(s.Token).Accept("application/json").MaxBody(maxRawBytes)
}

func (a *Account) user(path string) string {
	return a.opts.BaseURL + "/gmail/v1/users/me" + path
}

/*
get calls the API and decodes a successful answer.
*/
func (a *Account) get(ctx context.Context, s mailbox.Session, path string, out any) error {
	resp, err := a.api(s).Get(ctx, a.user(path))
	if err != nil {
		return fmt.Errorf("gmail %s: %w", strings.SplitN(path, "?", 2)[0], err)
	}

	if err := check(resp); err != nil {
		return err
	}

	return resp.JSON(out)
}

/*
Profile returns who signed in and the addresses Gmail lets them send as.
*/
func (a *Account) Profile(ctx context.Context, s mailbox.Session) (mailbox.Profile, error) {
	var profile struct {
		EmailAddress string
	}
	if err := a.get(ctx, s, "/profile", &profile); err != nil {
		return mailbox.Profile{}, err
	}

	var aliases struct {
		SendAs []struct {
			SendAsEmail        string
			VerificationStatus string
			IsPrimary          bool
		}
	}
	if err := a.get(ctx, s, "/settings/sendAs", &aliases); err != nil {
		return mailbox.Profile{}, err
	}

	out := mailbox.Profile{Email: profile.EmailAddress}

	for _, alias := range aliases.SendAs {
		if alias.VerificationStatus == "accepted" || alias.IsPrimary {
			out.Aliases = append(out.Aliases, alias.SendAsEmail)
		}
	}

	return out, nil
}

/*
cursor is the position in the history feed. PageToken is set while a page
is being walked, so a crash resumes where it stopped.
*/
type cursor struct {
	HistoryID string `json:"history_id"`
	Latest    string `json:"latest,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

func (c cursor) bytes() []byte {
	body, _ := json.Marshal(c)

	return body
}

/*
Cursor returns a cursor at this moment.
*/
func (a *Account) Cursor(ctx context.Context, s mailbox.Session) ([]byte, error) {
	id, err := a.historyID(ctx, s)
	if err != nil {
		return nil, err
	}

	return cursor{HistoryID: id}.bytes(), nil
}

func (a *Account) historyID(ctx context.Context, s mailbox.Session) (string, error) {
	var profile struct {
		HistoryID string
	}
	if err := a.get(ctx, s, "/profile", &profile); err != nil {
		return "", err
	}

	return profile.HistoryID, nil
}

/*
Changes returns one page of the history feed.
*/
func (a *Account) Changes(ctx context.Context, s mailbox.Session, raw []byte) (mailbox.Changes, error) {
	var at cursor
	if err := json.Unmarshal(raw, &at); err != nil || at.HistoryID == "" {
		return mailbox.Changes{}, mailbox.ErrCursorExpired
	}

	query := url.Values{
		"startHistoryId": {at.HistoryID}, "historyTypes": {"messageAdded"}, "maxResults": {pageSize},
	}
	if at.PageToken != "" {
		query.Set("pageToken", at.PageToken)
	}

	var page struct {
		History []struct {
			MessagesAdded []struct {
				Message struct {
					ID string `json:"id"`
				} `json:"message"`
			}
		} `json:"history"`
		NextPageToken string
		HistoryID     string
	}
	if err := a.get(ctx, s, "/history?"+query.Encode(), &page); err != nil {
		return mailbox.Changes{}, cursorError(err)
	}

	ids := []string{}
	for _, entry := range page.History {
		for _, added := range entry.MessagesAdded {
			ids = append(ids, added.Message.ID)
		}
	}

	items, err := a.classify(ctx, s, ids)
	if err != nil {
		return mailbox.Changes{}, err
	}

	if page.NextPageToken != "" {
		next := cursor{HistoryID: at.HistoryID, Latest: page.HistoryID, PageToken: page.NextPageToken}

		return mailbox.Changes{Items: items, Cursor: next.bytes(), More: true}, nil
	}

	return mailbox.Changes{Items: items, Cursor: cursor{HistoryID: page.HistoryID}.bytes()}, nil
}

/*
cursorError says that a history id Gmail answers not found for is stale.
*/
func cursorError(err error) error {
	if isNotFound(err) {
		return mailbox.ErrCursorExpired
	}

	return err
}

func isNotFound(err error) bool {
	status, ok := errors.AsType[*statusError](err)

	return ok && status.status == http.StatusNotFound
}

/*
classify reads the labels of each message and keeps mail worth reading,
sent mail first.
*/
func (a *Account) classify(ctx context.Context, s mailbox.Session, ids []string) ([]mailbox.Change, error) {
	items := []mailbox.Change{}

	for _, id := range ids {
		var message struct {
			ID       string `json:"id"`
			ThreadID string
			LabelIDs []string
		}

		err := a.get(ctx, s, "/messages/"+url.PathEscape(id)+"?format=minimal", &message)
		if isNotFound(err) {
			continue
		}

		if err != nil {
			return nil, err
		}

		folder, keep := a.folderOf(message.LabelIDs)
		if keep {
			items = append(items, mailbox.Change{ID: message.ID, ThreadID: message.ThreadID, Folder: folder})
		}
	}

	slices.SortStableFunc(items, func(x, y mailbox.Change) int { return rank(x.Folder) - rank(y.Folder) })

	return items, nil
}

func rank(folder mailbox.Folder) int {
	if folder == mailbox.FolderSent {
		return 0
	}

	return 1
}

func (a *Account) folderOf(labels []string) (mailbox.Folder, bool) {
	for _, skip := range skipLabels() {
		if slices.Contains(labels, skip) {
			return "", false
		}
	}

	switch {
	case slices.Contains(labels, sentLabel):
		return mailbox.FolderSent, true
	case slices.Contains(labels, a.opts.ReceivedLabel):
		return mailbox.FolderReceived, true
	default:
		return "", false
	}
}

/*
Since lists mail after a time, a day early so nothing falls between, and
gives a cursor taken before listing so nothing newer is missed.
*/
func (a *Account) Since(ctx context.Context, s mailbox.Session, since time.Time) (mailbox.Changes, error) {
	id, err := a.historyID(ctx, s)
	if err != nil {
		return mailbox.Changes{}, err
	}

	after := strconv.FormatInt(since.Add(-listDayMargin).Unix(), 10)
	query := url.Values{"q": {"after:" + after}, "maxResults": {"500"}}

	var list struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := a.get(ctx, s, "/messages?"+query.Encode(), &list); err != nil {
		return mailbox.Changes{}, err
	}

	ids := make([]string, 0, len(list.Messages))
	for _, message := range list.Messages {
		ids = append(ids, message.ID)
	}

	items, err := a.classify(ctx, s, ids)
	if err != nil {
		return mailbox.Changes{}, err
	}

	return mailbox.Changes{Items: items, Cursor: cursor{HistoryID: id}.bytes()}, nil
}

/*
Raw returns the raw message.
*/
func (a *Account) Raw(ctx context.Context, s mailbox.Session, id string) ([]byte, error) {
	var message struct {
		Raw string `json:"raw"`
	}
	if err := a.get(ctx, s, "/messages/"+url.PathEscape(id)+"?format=raw", &message); err != nil {
		return nil, err
	}

	raw, err := base64.URLEncoding.DecodeString(message.Raw)
	if err != nil {
		return nil, fmt.Errorf("decode gmail raw message: %w", err)
	}

	return raw, nil
}

/*
Verdicts reads the header Gmail wrote on top, and only when Google's mail
server wrote it.
*/
func (a *Account) Verdicts(raw []byte) mailbox.Verdicts {
	verdicts, _ := webhook.TopAuthResults(raw, authServer)

	return verdicts
}

/*
Send sends the raw message and reads back the Message-ID Gmail kept.
*/
func (a *Account) Send(ctx context.Context, s mailbox.Session, in mailbox.SendInput) (mailbox.Sent, error) {
	body, contentType, err := sendBody(in)
	if err != nil {
		return mailbox.Sent{}, err
	}

	resp, err := a.api(s).WithHeader("Content-Type", contentType).
		Post(ctx, a.opts.BaseURL+"/upload/gmail/v1/users/me/messages/send?uploadType=multipart", body)
	if err != nil {
		return mailbox.Sent{}, fmt.Errorf("gmail send: %w", err)
	}

	if err := checkSend(resp); err != nil {
		return mailbox.Sent{}, err
	}

	var sent struct {
		ID       string `json:"id"`
		ThreadID string
	}
	if err := resp.JSON(&sent); err != nil {
		return mailbox.Sent{}, err
	}

	return mailbox.Sent{
		ProviderID: sent.ID, ThreadID: sent.ThreadID, RFCMessageID: a.messageID(ctx, s, sent.ID),
	}, nil
}

/*
messageID reads the Message-ID of a sent message. Failing here would make
a sent message look unsent, so a missing id is left to sent sync.
*/
func (a *Account) messageID(ctx context.Context, s mailbox.Session, id string) string {
	var message struct {
		Payload struct {
			Headers []struct{ Name, Value string } `json:"headers"`
		} `json:"payload"`
	}

	query := "?format=metadata&metadataHeaders=Message-Id"
	if err := a.get(ctx, s, "/messages/"+url.PathEscape(id)+query, &message); err != nil {
		return ""
	}

	for _, header := range message.Payload.Headers {
		if strings.EqualFold(header.Name, "Message-Id") {
			return strings.Trim(header.Value, "<> ")
		}
	}

	return ""
}

/*
sendBody builds the multipart upload: the thread as JSON, then the raw
message.
*/
func sendBody(in mailbox.SendInput) ([]byte, string, error) {
	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	metadata, err := json.Marshal(map[string]string{"threadId": in.ThreadID})
	if err != nil {
		return nil, "", fmt.Errorf("gmail send metadata: %w", err)
	}

	if in.ThreadID == "" {
		metadata = []byte("{}")
	}

	for _, part := range []struct {
		contentType string
		content     []byte
	}{
		{"application/json; charset=UTF-8", metadata},
		{"message/rfc822", in.Raw},
	} {
		w, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {part.contentType}})
		if err != nil {
			return nil, "", fmt.Errorf("gmail send part: %w", err)
		}

		if _, err := w.Write(part.content); err != nil {
			return nil, "", fmt.Errorf("gmail send part: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("gmail send body: %w", err)
	}

	return body.Bytes(), "multipart/related; boundary=" + writer.Boundary(), nil
}

/*
Watch starts or renews announcements on the topic.
*/
func (a *Account) Watch(ctx context.Context, s mailbox.Session, in mailbox.WatchInput) (mailbox.Watching, error) {
	resp, err := a.api(s).Post(ctx, a.user("/watch"), map[string]any{
		"topicName": in.Topic, "labelIds": []string{a.opts.ReceivedLabel, sentLabel},
		"labelFilterBehavior": "include",
	})
	if err != nil {
		return mailbox.Watching{}, fmt.Errorf("gmail watch: %w", err)
	}

	if err := check(resp); err != nil {
		return mailbox.Watching{}, err
	}

	var watch struct {
		Expiration string `json:"expiration"`
	}
	if err := resp.JSON(&watch); err != nil {
		return mailbox.Watching{}, err
	}

	millis, err := strconv.ParseInt(watch.Expiration, 10, 64)
	if err != nil {
		return mailbox.Watching{}, fmt.Errorf("gmail watch expiration: %w", err)
	}

	return mailbox.Watching{ExpiresAt: time.UnixMilli(millis)}, nil
}

/*
StopWatch ends announcements.
*/
func (a *Account) StopWatch(ctx context.Context, s mailbox.Session, _ []byte) error {
	resp, err := a.api(s).Post(ctx, a.user("/stop"), nil)
	if err != nil {
		return fmt.Errorf("gmail stop: %w", err)
	}

	return check(resp)
}

/*
Revoke ends the grant at Google.
*/
func (a *Account) Revoke(ctx context.Context, refreshToken string) error {
	query := url.Values{"token": {refreshToken}}

	resp, err := a.client.New().Timeout(apiTimeout).Post(ctx, a.opts.RevokeURL+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("gmail revoke: %w", err)
	}

	if resp.Status != http.StatusOK {
		return fmt.Errorf("gmail revoke: status %d", resp.Status)
	}

	return nil
}
