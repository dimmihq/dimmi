/*
|--------------------------------------------------------------------------
| Sending To WhatsApp
|--------------------------------------------------------------------------
|
| How a reply goes out once the driver knows where: a template as one
| message, or text in parts and then each file as a message of its own.
| Before a reply the customer's newest message is marked read, as a
| person answering would.
|
*/

package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
markReadBeforeReply marks the customer's newest message read before a
reply, as a person answering would; it never holds the reply up.
*/
func (d *Driver) markReadBeforeReply(ctx context.Context, s sending, messageID string) {
	body := map[string]any{"messaging_product": "whatsapp", "status": "read", "message_id": messageID}
	_ = s.graph.Post(ctx, s.phone+"/messages", body, nil)
}

/*
sendTemplate sends the reply as the template it names. When Meta refuses
it, such as one paused or edited in WhatsApp Manager since dimmi copied
it, dimmi asks Meta for the template again at once and tells the teammate
what changed.
*/
func (d *Driver) sendTemplate(ctx context.Context, s sending, choice Choice) (contracts.SendResult, error) {
	t, ok, err := d.Places.Template(ctx, s.conn, choice.Name, choice.Language)
	if err != nil {
		return contracts.SendResult{}, err
	}

	if !ok {
		return contracts.SendResult{}, templateRefusal(0, choice, nil)
	}

	if !t.Sendable() {
		return contracts.SendResult{}, templateRefusal(0, choice, &t)
	}

	id, err := d.sendOne(ctx, s, "template", templatePayload(t, choice))
	if refused, isMeta := errors.AsType[*GraphError](err); isMeta && templateCode(refused.Code) {
		return contracts.SendResult{}, d.refused(ctx, s, choice, refused.Code)
	}

	if err != nil {
		return contracts.SendResult{}, MapError(err)
	}

	part := []Part{{ExternalID: id, Kind: "template"}}
	if err := d.Places.RecordParts(ctx, s.conn, s.msg.MessageID, part); err != nil {
		return contracts.SendResult{}, err
	}

	return resultOf(part), nil
}

/*
post sends the text in parts, then each file, remembering each part as
soon as Meta takes it and skipping those sent before.
*/
func (d *Driver) post(ctx context.Context, s sending, sent []Part) ([]Part, error) {
	parts, done := sent, map[int]bool{}
	for _, p := range sent {
		done[p.Index] = true
	}

	for _, p := range piecesOf(s.msg) {
		if done[p.index] {
			continue
		}

		part, err := d.sendPiece(ctx, s, p)
		if err != nil {
			return parts, err
		}

		if err := d.Places.RecordParts(ctx, s.conn, s.msg.MessageID, []Part{part}); err != nil {
			return parts, err
		}

		parts = append(parts, part)
	}

	return parts, nil
}

/*
piece is one WhatsApp message a reply becomes: a part of its text or one
of its files, at its place in the reply.
*/
type piece struct {
	index int
	text  string
	file  *contracts.OutboundAttachment
}

/*
piecesOf cuts the reply into the messages it becomes: its text in parts
WhatsApp takes, then each file.
*/
func piecesOf(msg contracts.OutboundMessage) []piece {
	var out []piece

	if body := ToWhatsApp(msg.BodyMarkdown); body != "" {
		for _, chunk := range Split(body, MaxBodyChars) {
			out = append(out, piece{index: len(out), text: chunk})
		}
	}

	for i := range msg.Attachments {
		out = append(out, piece{index: len(out), file: &msg.Attachments[i]})
	}

	return out
}

/*
sendPiece sends one piece of the reply.
*/
func (d *Driver) sendPiece(ctx context.Context, s sending, p piece) (Part, error) {
	if p.file != nil {
		return d.sendFile(ctx, s, *p.file, p.index)
	}

	id, err := d.message(ctx, s, "text", map[string]any{"body": p.text, "preview_url": hasLink(p.text)})

	return Part{ExternalID: id, Index: p.index, Kind: "text"}, err
}

/*
sendFile uploads one file and sends it as the kind of message WhatsApp
shows it best as.
*/
func (d *Driver) sendFile(ctx context.Context, s sending, file contracts.OutboundAttachment, index int) (Part, error) {
	kind := mediaKind(file.ContentType, file.SizeBytes)

	content, err := file.Open(ctx)
	if err != nil {
		return Part{}, fmt.Errorf("open reply file: %w", err)
	}
	defer func() { _ = content.Close() }()

	mediaID, err := s.graph.Upload(ctx, s.phone, httpclient.FilePart{
		Filename: file.Filename, ContentType: file.ContentType, Content: content,
	})
	if err != nil {
		return Part{}, MapError(err)
	}

	media := map[string]any{"id": mediaID}
	if kind == "document" {
		media["filename"] = file.Filename
	}

	id, err := d.message(ctx, s, kind, media)
	if err != nil {
		return Part{}, err
	}

	return Part{ExternalID: id, Index: index, Kind: "file"}, nil
}

/*
mediaKind is how WhatsApp shows a file: a picture or video it plays,
audio it plays, anything else a document to download.
*/
func mediaKind(contentType string, size int64) string {
	switch {
	case (contentType == "image/jpeg" || contentType == "image/png") && size <= maxImageBytes:
		return "image"
	case (contentType == "video/mp4" || contentType == "video/3gpp") && size <= maxVideoBytes:
		return "video"
	case strings.HasPrefix(contentType, "audio/") && size <= maxVideoBytes:
		return "audio"
	default:
		return "document"
	}
}

/*
message sends one message of the kind to the customer and returns
Meta's id for it.
*/
func (d *Driver) message(ctx context.Context, s sending, kind string, content any) (string, error) {
	id, err := d.sendOne(ctx, s, kind, content)
	if err != nil {
		return "", MapError(err)
	}

	return id, nil
}

/*
sendOne sends one message and returns its id, Meta's refusal unmapped.
*/
func (d *Driver) sendOne(ctx context.Context, s sending, kind string, content any) (string, error) {
	body := map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "type": kind, kind: content}

	if s.to.WaID != "" {
		body["to"] = s.to.WaID
	} else {
		body["recipient"] = s.to.UserID
	}

	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}

	if err := s.graph.Post(ctx, s.phone+"/messages", body, &out); err != nil {
		return "", err
	}

	if len(out.Messages) == 0 || out.Messages[0].ID == "" {
		return "", errors.New("whatsapp send: Meta returned no message id")
	}

	return out.Messages[0].ID, nil
}

/*
MarkRead tells the customer their message was read.
*/
func (d *Driver) MarkRead(ctx context.Context, conn models.ChannelConnection, messageID string) error {
	return d.read(ctx, conn, map[string]any{"message_id": messageID})
}

/*
ShowTyping marks the customer's message read and shows them the business
is writing, for about 25 seconds or until the reply arrives.
*/
func (d *Driver) ShowTyping(ctx context.Context, conn models.ChannelConnection, messageID string) error {
	return d.read(ctx, conn, map[string]any{
		"message_id": messageID, "typing_indicator": map[string]string{"type": "text"},
	})
}

/*
read sends a read receipt with the extras given, on the connection's
number.
*/
func (d *Driver) read(ctx context.Context, conn models.ChannelConnection, extras map[string]any) error {
	facts := d.FactsOf(conn)
	if facts.PhoneNumberID == "" || extras["message_id"] == "" {
		return nil
	}

	token, err := d.Credentials.Token(ctx, conn)
	if err != nil {
		return err
	}

	body := map[string]any{"messaging_product": "whatsapp", "status": "read"}
	maps.Copy(body, extras)

	return MapError(d.Graph(token).Post(ctx, facts.PhoneNumberID+"/messages", body, nil))
}

/*
hasLink reports whether the text holds a web address, so WhatsApp shows
its preview.
*/
func hasLink(text string) bool {
	return strings.Contains(text, "https://") || strings.Contains(text, "http://")
}
