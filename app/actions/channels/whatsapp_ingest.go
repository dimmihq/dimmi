/*
|--------------------------------------------------------------------------
| WhatsApp Messages Into The Inbox
|--------------------------------------------------------------------------
|
| A customer's WhatsApp message becomes an inbound message for the inbox:
| its writer as a contact known by their business-scoped user id and, when
| WhatsApp shares it, their phone number, both proven by Meta; its text as
| Markdown; its file fetched from Meta right away, since Meta serves it
| only for minutes; a place or a contact card as a line of text. Each
| message is recognized by Meta's id, so a retry never lands twice.
|
| A file larger than the inbox imports is left out; the message says a
| file was too large instead.
|
| Every message reopens the customer's 24-hour window, in the same
| transaction that stores it, so a reply written a moment later already
| sees the window open. A reaction is not a message: it changes nothing.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
errMediaTooLarge is a customer's file larger than the inbox imports.
*/
var errMediaTooLarge = errors.New("whatsapp media too large to import")

/*
errTemplatesStale says Meta could not be asked for the templates now.
*/
var errTemplatesStale = errors.New("whatsapp templates could not be refreshed")

/*
whatsappNameLimit bounds a customer's WhatsApp name.
*/
const whatsappNameLimit = 80

/*
ingest hands one customer message to the inbox, reporting false for one
that is not a message, such as a reaction.
*/
func (w *WhatsApp) ingest(
	ctx context.Context, conn models.ChannelConnection, v wadriver.Messages, m wadriver.Message,
) (bool, error) {
	if m.ID == "" || m.Type == "reaction" || m.Type == "system" || m.Type == "ephemeral" || album(m) {
		return false, nil
	}

	who := writerOf(v, m)
	key := wadriver.ThreadKey(who.UserID, who.WaID)
	if key == "" {
		return false, nil
	}

	body, files, err := w.content(ctx, conn, m)
	if err != nil {
		return false, err
	}

	sentAt := m.SentAt(w.Clock.Now())
	msg := contracts.InboundMessage{
		ConnectionID: conn.ID, DedupeKey: wadriver.DedupeKey(m.ID), ExternalThreadKey: key,
		Sender: senderOf(who), Body: body, BodyFormat: enums.BodyFormatMarkdown, Attachments: files,
		ChannelMessageID: m.ID, SentAt: sentAt,
		Source:   map[string]any{"channel": map[string]any{"type": wadriver.Type}},
		Metadata: map[string]any{"type": m.Type, "phone_number_id": v.Metadata.PhoneNumberID},
		AfterPersist: func(ctx context.Context, tx *database.Tx, conv models.Conversation, _ models.Message) error {
			return w.openWindow(ctx, tx, arrival{conn: conn, contactID: conv.ContactID, messageID: m.ID, at: sentAt})
		},
	}

	if m.Context != nil && m.Context.ID != "" {
		msg.ReplyToChannelMessageID = m.Context.ID
	}

	_, err = w.Receiver.ReceiveInboundMessage(ctx, msg)
	if errors.Is(err, conversations.ErrConnectionGone) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("receive whatsapp message: %w", err)
	}

	return true, nil
}

/*
writer is who wrote a message: their user id, their phone's WhatsApp id
when shared, and their name.
*/
type writer struct {
	UserID string
	WaID   string
	Name   string
}

/*
writerOf finds who wrote the message among the change's contacts.
*/
func writerOf(v wadriver.Messages, m wadriver.Message) writer {
	who := writer{UserID: m.FromUserID, WaID: m.From}

	for _, c := range v.Contacts {
		matches := (c.UserID != "" && c.UserID == who.UserID) || (c.WaID != "" && c.WaID == who.WaID)
		if matches || len(v.Contacts) == 1 {
			who.UserID = firstOf(who.UserID, c.UserID)
			who.WaID = firstOf(who.WaID, c.WaID)
			who.Name = cleanName(firstOf(c.Profile.Name, c.Profile.Username))

			break
		}
	}

	return who
}

/*
senderOf is the writer as the inbox's contact: known by their user id
and their phone, both proven by Meta.
*/
func senderOf(who writer) contracts.InboundSender {
	var claims []contracts.IdentityClaim

	if who.UserID != "" {
		claims = append(claims, contracts.IdentityClaim{
			Type: enums.IdentityTypeWhatsAppUser, Value: who.UserID, Verified: true,
		})
	}

	if who.WaID != "" {
		claims = append(claims, contracts.IdentityClaim{
			Type: enums.IdentityTypePhone, Value: wadriver.PhoneOf(who.WaID), Verified: true,
		})
	}

	return contracts.InboundSender{Identities: claims, Name: who.Name, Kind: enums.ContactKindUser}
}

/*
content reads the message's text and file.
*/
func (w *WhatsApp) content(ctx context.Context, conn models.ChannelConnection, m wadriver.Message) (
	string, []contracts.InboundAttachment, error,
) {
	if media := m.Media(); media != nil {
		file, err := w.attachment(ctx, conn, m, media)
		if errors.Is(err, errMediaTooLarge) {
			return strings.TrimSpace(wadriver.ToMarkdown(media.Caption) + "\n\n_A file too large to import._"), nil, nil
		}

		if err != nil {
			return "", nil, err
		}

		return wadriver.ToMarkdown(media.Caption), []contracts.InboundAttachment{file}, nil
	}

	return textOf(m), nil, nil
}

/*
textOf reads a message that carries no file as Markdown.
*/
func textOf(m wadriver.Message) string {
	switch {
	case m.Text != nil:
		return wadriver.ToMarkdown(m.Text.Body)
	case m.Interactive != nil && m.Interactive.ButtonReply != nil:
		return m.Interactive.ButtonReply.Title
	case m.Interactive != nil && m.Interactive.ListReply != nil:
		return m.Interactive.ListReply.Title
	case m.Button != nil:
		return firstOf(m.Button.Text, m.Button.Payload)
	case m.Location != nil:
		return locationText(*m.Location)
	case len(m.Contacts) > 0:
		return cardsText(m.Contacts)
	case m.Unsupported != nil && m.Unsupported.Type != "":
		return "_The customer sent a WhatsApp " + strings.ReplaceAll(m.Unsupported.Type, "_", " ") +
			" that WhatsApp does not pass on to dimmi._"
	default:
		return "_The customer sent a WhatsApp message that WhatsApp does not pass on to dimmi._"
	}
}

/*
album reports the notice WhatsApp sends before an album's pictures,
which arrive after it one by one.
*/
func album(m wadriver.Message) bool {
	return m.Type == "unsupported" && m.Unsupported != nil && m.Unsupported.Type == "album"
}

/*
locationText shows a shared place with a map link.
*/
func locationText(l wadriver.Location) string {
	lat := strconv.FormatFloat(l.Latitude, 'f', 6, 64)
	lng := strconv.FormatFloat(l.Longitude, 'f', 6, 64)
	name := strings.TrimSpace(strings.Join(nonEmpty(l.Name, l.Address), ", "))

	link := "https://maps.google.com/?q=" + lat + "," + lng
	if name == "" {
		return "Shared a location: " + link
	}

	return "Shared a location: " + name + "\n" + link
}

/*
cardsText shows shared contact cards as their names and numbers.
*/
func cardsText(cards []wadriver.SharedCard) string {
	lines := make([]string, 0, len(cards))

	for _, c := range cards {
		var phones []string
		for _, p := range c.Phones {
			phones = append(phones, p.Phone)
		}

		lines = append(lines, "- "+strings.TrimSpace(c.Name.FormattedName+" "+strings.Join(phones, ", ")))
	}

	return "Shared contacts:\n" + strings.Join(lines, "\n")
}

/*
attachment fetches the message's file from Meta, which serves it only
for minutes, and names it.
*/
func (w *WhatsApp) attachment(
	ctx context.Context, conn models.ChannelConnection, m wadriver.Message, media *wadriver.MediaRef,
) (contracts.InboundAttachment, error) {
	token, err := w.Token(ctx, conn)
	if err != nil {
		return contracts.InboundAttachment{}, err
	}

	graph := w.graph(token)

	info, err := graph.MediaInfo(ctx, media.ID)
	if err != nil {
		return contracts.InboundAttachment{}, fmt.Errorf("find whatsapp media: %w", wadriver.MapError(err))
	}

	if info.Size > w.Config.MaxMediaBytes {
		return contracts.InboundAttachment{}, errMediaTooLarge
	}

	contentType := firstOf(info.MimeType, media.MimeType, "application/octet-stream")
	name := fileName(m, media, contentType)
	address, limit := info.URL, w.Config.MaxMediaBytes

	return contracts.InboundAttachment{
		Filename: name, ContentType: baseType(contentType), SizeBytes: info.Size,
		Open: func(ctx context.Context) (io.ReadCloser, error) {
			return graph.Download(ctx, address, limit)
		},
	}, nil
}

/*
fileName names a file: a document by its own name, anything else by its
kind and the message, with the extension its type has.
*/
func fileName(m wadriver.Message, media *wadriver.MediaRef, contentType string) string {
	if media.Filename != "" {
		return media.Filename
	}

	kind := m.Type
	if m.Type == "audio" && media.Voice {
		kind = "voice-note"
	}

	ext := commonExtensions()[baseType(contentType)]

	suffix := m.ID
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}

	return "whatsapp-" + kind + "-" + strings.Trim(suffix, "=") + ext
}

/*
commonExtensions are the extensions of what WhatsApp sends; a file of
another type keeps no extension in its name.
*/
func commonExtensions() map[string]string {
	return map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif", "video/mp4": ".mp4",
		"audio/ogg": ".ogg", "audio/mpeg": ".mp3", "audio/mp4": ".m4a", "audio/aac": ".aac", "audio/amr": ".amr",
		"video/3gpp": ".3gp", "application/pdf": ".pdf", "text/plain": ".txt",
		"application/msword":       ".doc",
		"application/vnd.ms-excel": ".xls", "application/vnd.ms-powerpoint": ".ppt",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	}
}

/*
baseType drops a content type's parameters, such as a codec.
*/
func baseType(contentType string) string {
	t, _, _ := strings.Cut(contentType, ";")

	return strings.TrimSpace(t)
}

/*
arrival is a customer's message on a connection: who sent it, Meta's id
for it and when it was sent.
*/
type arrival struct {
	conn      models.ChannelConnection
	contactID ids.UUID
	messageID string
	at        time.Time
}

/*
openWindow records the customer's message on the connection and opens
their window for the next 24 hours.
*/
func (w *WhatsApp) openWindow(ctx context.Context, tx *database.Tx, a arrival) error {
	_, err := models.New(tx).RecordChannelContactInbound(ctx, models.RecordChannelContactInboundParams{
		WorkspaceID: a.conn.WorkspaceID, ChannelConnectionID: a.conn.ID, ContactID: a.contactID, SentAt: a.at,
		WindowExpiresAt: a.at.Add(w.Config.Window), MessageID: &a.messageID, Now: w.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("open whatsapp window: %w", err)
	}

	return nil
}

/*
cleanName keeps a customer's name as plain text: control and direction
characters out, at most 80 characters.
*/
func cleanName(name string) string {
	var b strings.Builder

	for _, r := range name {
		if r < 0x20 || r == 0x7f || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			continue
		}

		b.WriteRune(r)
	}

	return wadriver.Truncate(strings.TrimSpace(b.String()), whatsappNameLimit)
}

/*
firstOf returns the first value that is not empty.
*/
func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}

/*
nonEmpty keeps the values that are not empty.
*/
func nonEmpty(values ...string) []string {
	var out []string

	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}

	return out
}
