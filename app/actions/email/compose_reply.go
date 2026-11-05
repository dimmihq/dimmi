/*
|--------------------------------------------------------------------------
| Composing A Reply
|--------------------------------------------------------------------------
|
| Who a reply goes to, how it threads and how it is signed. It goes to
| the conversation's contact, with the people copied on the thread, or
| to whoever the request named. The workspace's own addresses are never
| among them, since mailing itself would loop.
|
| It answers the customer's latest message, references the thread from
| its root, and carries the conversation's reply token for the address
| it leaves from. Its sender reads as the teammate from the team, and
| the customer's last message is quoted below it.
|
*/

package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxQuoteLength bounds the quoted previous message.
*/
const maxQuoteLength = 2000

/*
sendOptions are the email options a reply request may carry: other
recipients, another address to send from, and false in Signature to
send it unsigned.
*/
type sendOptions struct {
	To            []string `json:"to"`
	Cc            []string `json:"cc"`
	Bcc           []string `json:"bcc"`
	FromAddressID string   `json:"from_address_id"`
	Signature     *bool    `json:"signature"`
}

/*
compose builds the reply's draft.
*/
func (a *SendEmailMessage) compose(
	ctx context.Context, q *models.Queries, out *outbound, previous models.OutboundEmail,
) error {
	to, cc, err := a.recipients(ctx, q, out)
	if err != nil {
		return err
	}

	thread, err := q.EnsureEmailThread(ctx, models.EnsureEmailThreadParams{
		WorkspaceID: out.msg.WorkspaceID, ConversationID: out.msg.ConversationID, EmailAddressID: &out.address.ID,
		ReplyToken: inbound.NewReplyToken(), NormalizedSubject: optional(inbound.NormalizeSubject(out.msg.Subject)),
		Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("ensure email thread: %w", err)
	}

	refs, err := a.references(ctx, q, out)
	if err != nil {
		return err
	}

	sender, err := a.sender(ctx, q, out)
	if err != nil {
		return err
	}

	from := mail.Address{Name: senderName(out.address, out.msg.Author), Email: sender.Email}
	messageID := previous.MessageIDHeader
	if messageID == "" {
		messageID = inbound.MessageID(models.PublicID(models.PrefixMessage, out.msg.MessageID), domainOf(from.Email))
	}

	out.draft = inbound.Draft{
		From: from, To: to, Cc: cc, Subject: subjectOf(out.msg, refs), Text: out.msg.BodyText, HTML: out.msg.BodyHTML,
		MessageID: messageID, References: inbound.TrimReferences(refs.ids, a.Config.Outbound.ReferencesMax),
		InReplyTo: refs.inReplyTo, Automated: out.msg.Author.Type == enums.AuthorTypeAgent,
		LoopStamp: a.Stamps.Loop(out.msg.WorkspaceID.String(), messageID), Signing: sender.Signing,
	}

	if out.address.ReplyToMode == string(enums.ReplyToModeToken) {
		out.draft.ReplyTo = mail.Address{
			Name: from.Name, Email: inbound.ReplyAddress(out.address.ForwardingAddress, thread.ReplyToken),
		}
	}

	return a.dress(ctx, q, out)
}

/*
sender returns the address the reply leaves from: its own while its
domain is verified, its forwarding address otherwise.
*/
func (a *SendEmailMessage) sender(ctx context.Context, q *models.Queries, out *outbound) (Sender, error) {
	if a.Senders == nil {
		return Sender{Email: out.address.ForwardingAddress, State: SendingDefaultSender}, nil
	}

	return a.Senders.ResolveSender(ctx, q, out.msg.WorkspaceID, out.address)
}

/*
recipients returns who the reply goes to and who is copied, without the
workspace's own addresses or suppressed copies.
*/
func (a *SendEmailMessage) recipients(
	ctx context.Context, q *models.Queries, out *outbound,
) ([]mail.Address, []mail.Address, error) {
	options, err := readOptions(out.msg.Options)
	if err != nil {
		return nil, nil, err
	}

	to := slices.Clone(options.To)
	if len(to) == 0 && out.msg.Recipient.IdentityValue != "" {
		to = []string{out.msg.Recipient.IdentityValue}
	}

	cc := slices.Clone(options.Cc)
	if len(options.To) == 0 && len(cc) == 0 {
		for _, r := range out.msg.CC {
			cc = append(cc, r.IdentityValue)
		}
	}

	own, err := q.ListWorkspaceEmailIdentities(ctx, out.msg.WorkspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("list workspace addresses: %w", err)
	}

	to, cc = clean(to, own, nil), clean(cc, own, to)
	if len(to) == 0 {
		return nil, nil, &contracts.SendError{
			Code: "recipient_missing", Message: "Contact has no email address", Err: contracts.ErrRecipientUnreachable,
		}
	}

	suppressed, err := q.ListSuppressedEmails(ctx, models.ListSuppressedEmailsParams{
		WorkspaceID: out.msg.WorkspaceID, Emails: slices.Concat(to, cc), Now: a.Clock.Now(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list suppressed addresses: %w", err)
	}

	if slices.ContainsFunc(to, func(e string) bool { return slices.Contains(suppressed, e) }) {
		return nil, nil, &contracts.SendError{
			Code: "recipient_suppressed", Message: "The recipient's address bounced or complained before.",
			Err: contracts.ErrRecipientUnreachable,
		}
	}

	return addressList(to, out.msg.Recipient.Name), addressList(clean(cc, suppressed, nil), ""), nil
}

/*
threadRefs are the Message-IDs a reply threads on: every message of the
conversation, oldest first, and the one it answers.
*/
type threadRefs struct {
	ids       []string
	inReplyTo string
	replied   bool
}

/*
references returns the thread's Message-IDs. The reply answers the
customer's latest message, else the latest one sent.
*/
func (a *SendEmailMessage) references(ctx context.Context, q *models.Queries, out *outbound) (threadRefs, error) {
	rows, err := q.ListConversationRefs(ctx, models.ListConversationRefsParams{
		WorkspaceID: out.msg.WorkspaceID, ConversationID: out.msg.ConversationID,
	})
	if err != nil {
		return threadRefs{}, fmt.Errorf("list conversation refs: %w", err)
	}

	var refs threadRefs

	for _, row := range rows {
		refs.ids = append(refs.ids, row.RfcMessageID)
		if row.Kind == string(enums.EmailRefKindInbound) {
			refs.inReplyTo, refs.replied = row.RfcMessageID, true
		}
	}

	for _, id := range out.msg.ReplyTo {
		if !slices.Contains(refs.ids, id) {
			refs.ids = append(refs.ids, id)
		}
	}

	if refs.inReplyTo == "" && len(refs.ids) > 0 {
		refs.inReplyTo = refs.ids[len(refs.ids)-1]
	}

	return refs, nil
}

/*
quote puts the customer's latest message below the reply, when the
address quotes and there is one.
*/
func (a *SendEmailMessage) quote(ctx context.Context, q *models.Queries, out *outbound) error {
	if !models.ReadEmailAddressSettings(out.address.Settings).QuotePrevious {
		return nil
	}

	last, err := q.FindQuotableMessage(ctx, models.FindQuotableMessageParams{
		WorkspaceID: out.msg.WorkspaceID, ConversationID: out.msg.ConversationID,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && (last.Body == nil || *last.Body == "")) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find quotable message: %w", err)
	}

	zone, err := time.LoadLocation(out.workspace.Timezone)
	if err != nil {
		zone = time.UTC
	}

	text := *last.Body
	if runes := []rune(text); len(runes) > maxQuoteLength {
		text = string(runes[:maxQuoteLength]) + "…"
	}

	out.draft.Quote = &inbound.Quote{
		Header: inbound.QuoteHeader(last.CreatedAt, deref(last.Name), deref(last.Email), zone), Text: text,
	}

	return nil
}

/*
subjectOf returns the reply's subject: Re: and the conversation's
subject for a reply to the customer, the subject as it is for mail that
starts a thread.
*/
func subjectOf(msg contracts.OutboundMessage, refs threadRefs) string {
	if !refs.replied && msg.ExternalThreadKey == "" && strings.TrimSpace(msg.Subject) != "" {
		return strings.TrimSpace(msg.Subject)
	}

	return inbound.ReplySubject(msg.Subject)
}

/*
senderName returns how the reply is signed: the teammate from the
team, or only the team's name.
*/
func senderName(address models.EmailAddress, author contracts.OutboundAuthor) string {
	name := strings.TrimSpace(author.Name)
	if address.FromNameStyle == string(enums.FromNameStyleNameOnly) || name == "" ||
		author.Type == enums.AuthorTypeSystem {
		return inbound.DisplayName(address.DisplayName)
	}

	return inbound.DisplayName(name + " from " + address.DisplayName)
}

/*
clean lowercases and dedupes addresses, dropping invalid ones and those
in either exclusion list.
*/
func clean(addresses, excluded, also []string) []string {
	var out []string

	for _, address := range addresses {
		parsed, err := mail.ParseAddress(strings.TrimSpace(address))
		if err != nil {
			continue
		}

		email := lower(parsed.Email)
		if slices.Contains(out, email) || slices.Contains(excluded, email) || slices.Contains(also, email) {
			continue
		}

		out = append(out, email)
	}

	return out
}

/*
addressList turns addresses into mail addresses, the first one named.
*/
func addressList(emails []string, name string) []mail.Address {
	out := make([]mail.Address, 0, len(emails))
	for i, email := range emails {
		address := mail.Address{Email: email}
		if i == 0 {
			address.Name = inbound.DisplayName(name)
		}

		out = append(out, address)
	}

	return out
}

/*
readOptions decodes the email options of a reply request.
*/
func readOptions(raw []byte) (sendOptions, error) {
	var options sendOptions
	if len(raw) == 0 {
		return options, nil
	}

	var wrapped struct {
		Email *sendOptions `json:"email"`
	}

	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Email != nil {
		return *wrapped.Email, nil
	}

	if err := json.Unmarshal(raw, &options); err != nil {
		return options, &contracts.SendError{
			Code: "invalid_options", Message: "The email options cannot be read.", Err: contracts.ErrPermanent,
		}
	}

	return options, nil
}

/*
outboundKind returns the kind of email a reply is, by who wrote it.
*/
func outboundKind(msg contracts.OutboundMessage) enums.OutboundEmailKind {
	switch {
	case msg.Author.Type == enums.AuthorTypeAgent:
		return enums.OutboundEmailKindAIReply
	case msg.Author.Type == enums.AuthorTypeSystem:
		return enums.OutboundEmailKindSystem
	case msg.ExternalThreadKey == "":
		return enums.OutboundEmailKindNewConversation
	}

	return enums.OutboundEmailKindReply
}

/*
noSender is the failure of a reply the workspace has no address for.
*/
func noSender() error {
	return &contracts.SendError{
		Code: exceptions.ErrNoSenderAvailable.Code(), Message: exceptions.ErrNoSenderAvailable.Message(),
		Err: contracts.ErrPermanent,
	}
}

/*
emailsOf returns the addresses of mail addresses.
*/
func emailsOf(list []mail.Address) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.Email)
	}

	return out
}

/*
deref returns the text, or "" for none.
*/
func deref(text *string) string {
	if text == nil {
		return ""
	}

	return *text
}
