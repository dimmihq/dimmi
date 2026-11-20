/*
|--------------------------------------------------------------------------
| Connecting WhatsApp
|--------------------------------------------------------------------------
|
| A workspace connects a WhatsApp number in one of two ways. Through the
| install's own Meta app, by Meta's Embedded Signup popup: the popup
| hands back a code, which is exchanged for a token of the customer's
| business, and the account and number they picked. Or with a token of
| a Meta app of the workspace's own, a system user's that never expires:
| its WhatsApp accounts are read with it, and the admin picks a number.
|
| Everything is checked with Meta before anything is kept: the token,
| its permissions and the number. One number feeds one workspace;
| connecting it again here says so, and one another workspace holds is
| refused without naming it. Once connected, the app is subscribed to
| the account's webhooks and its templates are copied in.
|
*/

package channels

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Lengths of what a connection makes up: its number's PIN and the token
Meta echoes when it checks a webhook address.
*/
const (
	pinDigits        = 6
	verifyTokenChars = 32
)

/*
whatsappScopes are the permissions a token needs.
*/
func whatsappScopes() []string {
	return []string{"whatsapp_business_management", "whatsapp_business_messaging"}
}

/*
WhatsAppNumber is a number on a WhatsApp account, as Meta describes it.
*/
type WhatsAppNumber struct {
	ID                 string `json:"id"`
	DisplayPhoneNumber string `json:"display_phone_number"`
	VerifiedName       string `json:"verified_name"`
	NameStatus         string `json:"name_status"`
	QualityRating      string `json:"quality_rating"`
	MessagingLimit     string `json:"messaging_limit_tier"`
	PlatformType       string `json:"platform_type"`
	Connected          bool   `json:"-"`
}

/*
WhatsAppAccount is a WhatsApp account and its numbers.
*/
type WhatsAppAccount struct {
	ID         string
	Name       string
	BusinessID string
	Business   string
	Numbers    []WhatsAppNumber
}

/*
WhatsAppTokenInput is a token of a Meta app to read accounts with: the
token, the account to read, and for an app other than the install's
own, the app's secret, which signs its deliveries.
*/
type WhatsAppTokenInput struct {
	Membership authz.Membership
	Token      string
	AccountID  string
	AppSecret  string
}

/*
ConnectWhatsAppInput is a number to connect: the token and account it
is read with, the number picked, the inbox its conversations land in,
the default inbox when nil, and a name.
*/
type ConnectWhatsAppInput struct {
	WhatsAppTokenInput

	PhoneNumberID string
	InboxID       *ids.UUID
	Name          string
}

/*
WhatsAppSignupInput is what Meta's popup handed back: the code, the
address it was returned to, the account and number picked when Meta
said, and whether the number stays on the WhatsApp Business app as
well.
*/
type WhatsAppSignupInput struct {
	Membership    authz.Membership
	Code          string
	RedirectURI   string
	AccountID     string
	PhoneNumberID string
	BusinessID    string
	Coexistence   bool
	InboxID       *ids.UUID
}

/*
tokenInfo is what Meta says about a token: whose app it is and what it
may do.
*/
type tokenInfo struct {
	AppID  string
	Valid  bool
	Scopes []string
}

/*
Account reads the account the token names, with its numbers, marking
those already connected here.

Errors: [exceptions.ErrForbidden], [exceptions.ErrWhatsAppTokenRejected],
[exceptions.ErrWhatsAppAccountNotFound], [exceptions.ErrWhatsAppUnavailable].
*/
func (w *WhatsApp) Account(ctx context.Context, in WhatsAppTokenInput) (WhatsAppAccount, error) {
	if !in.Membership.Can("channels:manage") {
		return WhatsAppAccount{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, in.Membership.Workspace.ID.String())

	if _, _, err := w.checkToken(ctx, in.Token, in.AppSecret); err != nil {
		return WhatsAppAccount{}, err
	}

	account, err := w.readAccount(ctx, w.graph(in.Token), in.AccountID)
	if err != nil {
		return account, err
	}

	connected, err := w.connectedNumbers(ctx, in.Membership.Workspace.ID)
	for i, n := range account.Numbers {
		account.Numbers[i].Connected = slices.Contains(connected, n.ID)
	}

	return account, err
}

/*
Connect connects the number with a token of a Meta app.

Errors: as [WhatsApp.Account], and [exceptions.ErrWhatsAppNumberNotFound],
[exceptions.ErrChannelAlreadyConnected],
[exceptions.ErrChannelConnectedElsewhere].
*/
func (w *WhatsApp) Connect(ctx context.Context, in ConnectWhatsAppInput) (models.ChannelConnection, error) {
	if !in.Membership.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, in.Membership.Workspace.ID.String())

	appID, mode, err := w.checkToken(ctx, in.Token, in.AppSecret)
	if err != nil {
		return models.ChannelConnection{}, err
	}

	secrets := WhatsAppSecrets{AccessToken: in.Token}
	if mode == WhatsAppModeOwn {
		secrets.AppSecret, secrets.VerifyToken = in.AppSecret, randomToken()
	}

	return w.attach(ctx, attachment{
		m: in.Membership, token: in.Token, accountID: in.AccountID, phoneID: in.PhoneNumberID, inbox: in.InboxID,
		name: in.Name, facts: wadriver.Facts{Mode: mode, AppID: appID}, secrets: secrets,
	})
}

/*
Signup connects the number Meta's popup handed back: the code becomes
a token of the customer's business, and a number new to the Cloud API
is registered with a PIN of its own.

Errors: as [WhatsApp.Connect], and [exceptions.ErrWhatsAppSignupExpired],
[exceptions.ErrWhatsAppSignupUnavailable].
*/
func (w *WhatsApp) Signup(ctx context.Context, in WhatsAppSignupInput) (models.ChannelConnection, error) {
	if !in.Membership.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	if !w.Service.Configured() {
		return models.ChannelConnection{}, exceptions.ErrWhatsAppSignupUnavailable
	}

	ctx = tenancy.With(ctx, in.Membership.Workspace.ID.String())

	token, err := w.exchange(ctx, in.Code, in.RedirectURI)
	if err != nil {
		return models.ChannelConnection{}, err
	}

	if in.AccountID == "" || in.PhoneNumberID == "" {
		if in.AccountID, in.PhoneNumberID, err = w.granted(ctx, token, in.AccountID); err != nil {
			return models.ChannelConnection{}, err
		}
	}

	secrets := WhatsAppSecrets{AccessToken: token}
	if !in.Coexistence {
		secrets.PIN = randomPIN()
		if err := w.register(ctx, w.graph(token), in.PhoneNumberID, secrets.PIN); err != nil {
			return models.ChannelConnection{}, err
		}
	}

	return w.attach(ctx, attachment{
		m: in.Membership, token: token, accountID: in.AccountID, phoneID: in.PhoneNumberID, inbox: in.InboxID,
		facts: wadriver.Facts{
			Mode: WhatsAppModeShared, AppID: w.Service.AppID, BusinessID: in.BusinessID, Coexistence: in.Coexistence,
		},
		secrets: secrets,
	})
}

/*
attachment is a number on its way to being connected.
*/
type attachment struct {
	m         authz.Membership
	token     string
	accountID string
	phoneID   string
	inbox     *ids.UUID
	name      string
	facts     wadriver.Facts
	secrets   WhatsAppSecrets
}

/*
attach reads the number from Meta, registers it on the Cloud API when
it is not there yet, keeps the connection with its sealed secrets,
subscribes the app to the account and copies its templates.
*/
func (w *WhatsApp) attach(ctx context.Context, a attachment) (models.ChannelConnection, error) {
	graph := w.graph(a.token)

	account, err := w.readAccount(ctx, graph, a.accountID)
	if err != nil {
		return models.ChannelConnection{}, err
	}

	i := slices.IndexFunc(account.Numbers, func(n WhatsAppNumber) bool { return n.ID == a.phoneID })
	if i < 0 {
		return models.ChannelConnection{}, exceptions.ErrWhatsAppNumberNotFound
	}

	number := account.Numbers[i]
	if number.PlatformType != "" && number.PlatformType != "CLOUD_API" && a.secrets.PIN == "" && !a.facts.Coexistence {
		a.secrets.PIN = randomPIN()
		if err := w.register(ctx, graph, number.ID, a.secrets.PIN); err != nil {
			return models.ChannelConnection{}, err
		}
	}

	facts := a.facts
	facts.AccountID, facts.PhoneNumberID = account.ID, number.ID
	facts.BusinessID = cmp.Or(facts.BusinessID, account.BusinessID)
	facts.DisplayPhoneNumber, facts.VerifiedName = number.DisplayPhoneNumber, number.VerifiedName
	facts.NameStatus, facts.QualityRating, facts.MessagingLimit = number.NameStatus, number.QualityRating,
		number.MessagingLimit

	conn, err := w.store(ctx, a, facts)
	if err != nil {
		return conn, err
	}

	if err := graph.Post(ctx, account.ID+"/subscribed_apps", map[string]any{}, nil); err != nil {
		return conn, w.unavailable(err)
	}

	if err := w.SyncTemplates(ctx, conn); err != nil {
		w.Log.WarnContext(ctx, "whatsapp templates not synced",
			log.String("connection", conn.ID.String()), log.Err(err))
	}

	return conn, nil
}

/*
store keeps the connection, live, with its sealed secrets, refusing a
number already connected.
*/
func (w *WhatsApp) store(ctx context.Context, a attachment, facts wadriver.Facts) (models.ChannelConnection, error) {
	a.facts = facts

	config, err := json.Marshal(facts)
	if err != nil {
		return models.ChannelConnection{}, fmt.Errorf("encode whatsapp config: %w", err)
	}

	var conn models.ChannelConnection

	err = w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if conn, err = w.insert(ctx, q, a, config); err != nil {
			return err
		}

		sealed, err := w.seal(ctx, conn, a.secrets)
		if err != nil {
			return err
		}

		conn.CredentialsEncrypted = sealed

		return q.SealChannelConnectionCredentials(ctx, models.SealChannelConnectionCredentialsParams{
			CredentialsEncrypted: sealed, Now: w.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID,
		})
	})

	return conn, err
}

/*
insert adds the live connection of the attachment's number to the workspace,
refusing a number connected here or elsewhere already.
*/
func (w *WhatsApp) insert(ctx context.Context, q *models.Queries, a attachment, config json.RawMessage) (
	models.ChannelConnection, error,
) {
	ws, phoneID := a.m.Workspace.ID, a.facts.PhoneNumberID

	_, err := q.FindLiveChannelConnectionByExternalID(ctx, models.FindLiveChannelConnectionByExternalIDParams{
		WorkspaceID: ws, Type: wadriver.Type, ExternalID: &phoneID,
	})
	if err == nil {
		return models.ChannelConnection{}, exceptions.ErrChannelAlreadyConnected
	}

	if !errors.Is(err, database.ErrNotFound) {
		return models.ChannelConnection{}, err
	}

	inbox, err := inboxFor(ctx, q, ws, a.inbox)
	if err != nil {
		return models.ChannelConnection{}, err
	}

	conn, err := q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
		WorkspaceID: ws, InboxID: inbox, Type: wadriver.Type, Status: "active", ExternalID: &phoneID,
		Name: cmp.Or(a.name, a.facts.VerifiedName, a.facts.DisplayPhoneNumber, "WhatsApp"), Config: config,
		Now: w.Clock.Now(),
	})
	if database.IsUniqueViolation(err, "") {
		return conn, exceptions.ErrChannelConnectedElsewhere
	}

	if err != nil {
		return conn, fmt.Errorf("create whatsapp connection: %w", err)
	}

	return conn, nil
}

/*
inboxFor returns the inbox named, which must be the workspace's, or the
default one.
*/
func inboxFor(ctx context.Context, q *models.Queries, ws ids.UUID, named *ids.UUID) (ids.UUID, error) {
	if named == nil {
		inbox, err := q.FindDefaultInbox(ctx, ws)

		return inbox.ID, err
	}

	inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: ws, ID: *named})
	if errors.Is(err, database.ErrNotFound) || (err == nil && inbox.DeletedAt != nil) {
		return ids.Nil(), exceptions.ErrNotFound.OnField("inbox_id")
	}

	return inbox.ID, err
}

/*
checkToken asks Meta about the token: it must be valid, carry both
WhatsApp permissions, and belong to the install's own app or come with
its own app's secret. It returns the app and how the number connects.
*/
func (w *WhatsApp) checkToken(ctx context.Context, token, appSecret string) (string, string, error) {
	if strings.TrimSpace(token) == "" {
		return "", "", exceptions.ErrWhatsAppTokenRejected
	}

	info, err := w.debugToken(ctx, token)
	if err != nil {
		return "", "", err
	}

	for _, scope := range whatsappScopes() {
		if !slices.Contains(info.Scopes, scope) {
			return "", "", exceptions.ErrWhatsAppTokenRejected
		}
	}

	if w.Service.AppID != "" && info.AppID == w.Service.AppID && w.Service.AppSecret != "" {
		return info.AppID, WhatsAppModeShared, nil
	}

	if strings.TrimSpace(appSecret) == "" {
		return "", "", exceptions.ErrWhatsAppTokenRejected.With("hint", "app_secret_required")
	}

	return info.AppID, WhatsAppModeOwn, nil
}

/*
debugToken reads what Meta knows of the token, asking with the token
itself.
*/
func (w *WhatsApp) debugToken(ctx context.Context, token string) (tokenInfo, error) {
	var out struct {
		Data struct {
			AppID   string   `json:"app_id"`
			IsValid bool     `json:"is_valid"`
			Scopes  []string `json:"scopes"`
		} `json:"data"`
	}

	err := w.graph(token).Get(ctx, "debug_token", url.Values{"input_token": {token}}, &out)
	if refused, ok := errors.AsType[*wadriver.GraphError](err); ok && refused.Status < 500 {
		return tokenInfo{}, exceptions.ErrWhatsAppTokenRejected
	}

	if err != nil {
		return tokenInfo{}, w.unavailable(err)
	}

	if !out.Data.IsValid {
		return tokenInfo{}, exceptions.ErrWhatsAppTokenRejected
	}

	return tokenInfo{AppID: out.Data.AppID, Valid: true, Scopes: out.Data.Scopes}, nil
}

/*
readAccount reads the WhatsApp account and its numbers.
*/
func (w *WhatsApp) readAccount(ctx context.Context, graph *wadriver.Graph, accountID string) (WhatsAppAccount, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" || strings.ContainsAny(accountID, "/?#") {
		return WhatsAppAccount{}, exceptions.ErrWhatsAppAccountNotFound
	}

	var info struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Owner struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"owner_business_info"`
	}

	err := graph.Get(ctx, accountID, url.Values{"fields": {"id,name,owner_business_info"}}, &info)
	if refused, ok := errors.AsType[*wadriver.GraphError](err); ok && refused.Status < 500 {
		return WhatsAppAccount{}, exceptions.ErrWhatsAppAccountNotFound
	}

	if err != nil {
		return WhatsAppAccount{}, w.unavailable(err)
	}

	var numbers struct {
		Data []WhatsAppNumber `json:"data"`
	}

	fields := "id,display_phone_number,verified_name,name_status,quality_rating,messaging_limit_tier,platform_type"
	if err := graph.Get(ctx, accountID+"/phone_numbers", url.Values{"fields": {fields}}, &numbers); err != nil {
		return WhatsAppAccount{}, w.unavailable(err)
	}

	return WhatsAppAccount{
		ID: info.ID, Name: info.Name, BusinessID: info.Owner.ID, Business: info.Owner.Name, Numbers: numbers.Data,
	}, nil
}

/*
unavailable reports Meta not answering, keeping the cause for the logs.
*/
func (w *WhatsApp) unavailable(err error) error {
	return fmt.Errorf("%w: %w", exceptions.ErrWhatsAppUnavailable, err)
}

/*
randomPIN returns a six-digit PIN for a number's two-step verification.
*/
func randomPIN() string {
	return str.RandomDigits(pinDigits)
}

/*
randomToken returns a token Meta echoes when it checks a webhook address.
*/
func randomToken() string {
	return str.Random(verifyTokenChars)
}
