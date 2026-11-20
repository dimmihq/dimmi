/*
|--------------------------------------------------------------------------
| Sending
|--------------------------------------------------------------------------
|
| Each part of a reply is one call to the account's messages: who it is
| for, how Meta is told it may go, and the text or one file. A Page's
| file travels in the call itself. Instagram only takes an address it
| can fetch, so its files go as a link that works for a short while. A
| part Meta accepted is recorded before the next one goes, so a failure
| later never sends the earlier ones twice.
|
*/

package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/models"
)

/*
piece is one message a reply becomes: a part of its text or one file.
*/
type piece struct {
	text string
	file *contracts.OutboundAttachment
	kind string
}

/*
piecesOf cuts the reply into the messages it becomes, refusing a file
the platform does not take.
*/
func (d *Driver) piecesOf(msg contracts.OutboundMessage) ([]piece, error) {
	var out []piece

	body := strings.TrimSpace(msg.BodyText)
	for _, chunk := range d.split(body) {
		out = append(out, piece{text: chunk})
	}

	for i := range msg.Attachments {
		file := &msg.Attachments[i]

		kind, ok := d.Platform.kindOf(file.ContentType, file.SizeBytes)
		if !ok {
			return nil, &contracts.SendError{
				Code: CodeContentRejected, Message: d.unsendable(*file), Err: contracts.ErrContentRejected,
			}
		}

		out = append(out, piece{file: file, kind: kind})
	}

	return out, nil
}

/*
unsendable says why the platform does not take the file.
*/
func (d *Driver) unsendable(file contracts.OutboundAttachment) string {
	if kind, _ := d.Platform.kindOf(file.ContentType, 0); kind == "file" && d.Platform == Instagram {
		return "Instagram only takes pictures, video, audio and PDF files. " + file.Filename + " was not sent."
	}

	return file.Filename + " is too large for " + d.Platform.Name() + ": 8 MB for pictures, 25 MB for other files."
}

/*
split cuts text into messages the platform takes, at a line or a space
where it can.
*/
func (d *Driver) split(text string) []string {
	var out []string

	for text != "" {
		if d.Platform.fits(text) {
			return append(out, text)
		}

		cut := d.cut(text)
		out = append(out, strings.TrimSpace(text[:cut]))
		text = strings.TrimSpace(text[cut:])
	}

	return out
}

/*
cut is where the first message of a text too long for one ends: the
last line break or space that still fits, or the last whole character.
*/
func (d *Driver) cut(text string) int {
	end := 0

	for i := range text {
		if !d.Platform.fits(text[:i]) {
			break
		}

		end = i
	}

	if end == 0 {
		_, size := utf8.DecodeRuneInString(text)

		return size
	}

	if at := strings.LastIndexAny(text[:end], "\n "); at > end/2 {
		return at
	}

	return end
}

/*
post sends the pieces not sent yet, recording each as Meta accepts it.
*/
func (d *Driver) post(ctx context.Context, s sending, pieces []piece, sent []Part) ([]Part, error) {
	parts := append([]Part(nil), sent...)

	for index := len(sent); index < len(pieces); index++ {
		part, err := d.sendPiece(ctx, s, pieces[index])
		if err != nil {
			return parts, MapError(err)
		}

		part.Index = index
		if err := d.Places.RecordParts(ctx, s.conn, s.msg.MessageID, []Part{part}); err != nil {
			return parts, err
		}

		parts = append(parts, part)
	}

	return parts, nil
}

/*
sendPiece sends one piece and returns the part it became.
*/
func (d *Driver) sendPiece(ctx context.Context, s sending, p piece) (Part, error) {
	if p.file == nil {
		id, err := d.message(ctx, s, map[string]any{"text": p.text})

		return Part{ExternalID: id, Kind: "text"}, err
	}

	if d.Platform == Instagram {
		if p.file.SignedURL == "" {
			return Part{}, refuse(CodeContentRejected, p.file.Filename+" has no address Instagram can fetch.")
		}

		id, err := d.message(ctx, s, attachment(p.kind, map[string]any{"url": p.file.SignedURL}))

		return Part{ExternalID: id, Kind: "file"}, err
	}

	id, err := d.upload(ctx, s, p)

	return Part{ExternalID: id, Kind: "file"}, err
}

func attachment(kind string, payload map[string]any) map[string]any {
	return map[string]any{"attachment": map[string]any{"type": kind, "payload": payload}}
}

/*
envelope is what every send says beside its message: who it is for and
how Meta is told it may go.
*/
func (s sending) envelope() map[string]any {
	out := map[string]any{"recipient": map[string]string{"id": s.to.Customer}, "messaging_type": "RESPONSE"}
	if s.tagged {
		out["messaging_type"], out["tag"] = "MESSAGE_TAG", "HUMAN_AGENT"
	}

	return out
}

/*
sentMessage is Meta's answer to a send.
*/
type sentMessage struct {
	ID string `json:"message_id"`
}

var errNoMessageID = errors.New("social send: Meta returned no message id")

/*
message sends one message and returns Meta's id for it.
*/
func (d *Driver) message(ctx context.Context, s sending, message map[string]any) (string, error) {
	body := s.envelope()
	body["message"] = message

	var out sentMessage
	if err := s.graph.Post(ctx, s.account+"/messages", body, &out); err != nil {
		return "", err
	}

	if out.ID == "" {
		return "", errNoMessageID
	}

	return out.ID, nil
}

/*
upload sends a Page's file in the call itself.
*/
func (d *Driver) upload(ctx context.Context, s sending, p piece) (string, error) {
	content, err := p.file.Open(ctx)
	if err != nil {
		return "", fmt.Errorf("open reply file: %w", err)
	}

	defer func() { _ = content.Close() }()

	fields := map[string]string{}

	for key, value := range s.envelope() {
		text, isText := value.(string)
		if !isText {
			raw, _ := json.Marshal(value)
			text = string(raw)
		}

		fields[key] = text
	}

	raw, _ := json.Marshal(attachment(p.kind, map[string]any{"is_reusable": false}))
	fields["message"] = string(raw)

	var out sentMessage

	err = s.graph.Multipart(ctx, s.account+"/messages", meta.Form{Fields: fields, File: httpclient.FilePart{
		Field: "filedata", Filename: p.file.Filename, ContentType: p.file.ContentType, Content: content,
	}}, &out)
	if err != nil {
		return "", err
	}

	if out.ID == "" {
		return "", errNoMessageID
	}

	return out.ID, nil
}

/*
act tells the customer the thread was seen or that a reply is being
written.
*/
func (d *Driver) act(ctx context.Context, s sending, action string) error {
	return s.graph.Post(ctx, s.account+"/messages", map[string]any{
		"recipient": map[string]string{"id": s.to.Customer}, "sender_action": action,
	}, nil)
}

/*
ShowTyping shows the customer that a reply is being written, for about
20 seconds or until it arrives.
*/
func (d *Driver) ShowTyping(ctx context.Context, conn models.ChannelConnection, customer string) error {
	facts := d.FactsOf(conn)
	if facts.AccountID == "" || customer == "" {
		return nil
	}

	token, err := d.Credentials.Token(ctx, conn)
	if err != nil {
		return err
	}

	s := sending{account: facts.AccountID, graph: d.Graph(token), to: Target{Customer: customer}}

	return MapError(d.act(ctx, s, "typing_on"))
}
