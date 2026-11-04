//go:build emailmatrix

/*
|--------------------------------------------------------------------------
| Mailbox Scenarios
|--------------------------------------------------------------------------
|
| Each host goes through the same conversation. A customer writes in, a
| teammate replies and the host must file that reply in the inbox with SPF,
| DKIM and DMARC passing and nothing tracked. The customer answers and the
| answer must land in the same conversation inside a minute with the quote
| gone. Then a reply reaches a second mailbox on Cc, and its reply-all must land
| in the thread too. A step that depends on an earlier one is blocked when the
| earlier one did not pass.
|
*/

package emailmatrix

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"
)

const (
	scenarioNewMail  = "new mail makes a conversation"
	scenarioReply    = "reply reaches the inbox authenticated"
	scenarioAnswer   = "answer lands in the same conversation"
	scenarioReplyAll = "reply all from cc lands in the thread"
)

/*
harness runs scenarios. Wait bounds how long mail may take to appear anywhere;
AnswerBar is the slower promise for an answer landing in a conversation; Poll
is how often it looks.
*/
type harness struct {
	Clock      clock.Clock
	Product    product
	Mailboxes  []mailbox
	Forwarding string
	AddressID  string
	Wait       time.Duration
	AnswerBar  time.Duration
	Poll       time.Duration
}

/*
thread is what one host's scenarios share as they go.
*/
type thread struct {
	box          mailbox
	peer         mailbox
	marker       string
	subject      string
	conversation string
	contact      string
	replyMarker  string
	messageID    string
}

func (h *harness) runMatrix(ctx context.Context) report {
	var results []result

	for index, box := range h.Mailboxes {
		peer := h.Mailboxes[(index+1)%len(h.Mailboxes)]
		results = append(results, h.runHost(ctx, box, peer)...)
	}

	return report{Results: results}
}

func (h *harness) runHost(ctx context.Context, box, peer mailbox) []result {
	marker := "matrix-" + str.Random(12)
	state := &thread{box: box, peer: peer, marker: marker, subject: "Question " + marker}

	steps := []struct {
		name string
		run  func(context.Context, *thread) error
	}{
		{scenarioNewMail, h.newMail},
		{scenarioReply, h.reply},
		{scenarioAnswer, h.answer},
		{scenarioReplyAll, h.replyAll},
	}

	results := make([]result, 0, len(steps))
	stopped := ""

	for _, step := range steps {
		if stopped != "" {
			results = append(results, result{
				Host: box.Host, Scenario: step.name, Status: statusBlocked, Detail: "needs: " + stopped,
			})

			continue
		}

		started := h.Clock.Now()
		err := step.run(ctx, state)
		entry := resultOf(box.Host, step.name, err)
		entry.Seconds = h.Clock.Since(started).Seconds()
		results = append(results, entry)

		if entry.Status != statusPass {
			stopped = step.name
		}
	}

	return results
}

func (h *harness) newMail(ctx context.Context, t *thread) error {
	err := t.box.Out.Send(ctx, mail.Message{
		From:    mail.Address{Email: t.box.Address},
		To:      []mail.Address{{Email: h.Forwarding}},
		Cc:      []mail.Address{{Email: t.peer.Address}},
		Subject: t.subject,
		Text:    "Hello, this is " + t.marker,
	})
	if err != nil {
		return fmt.Errorf("send new mail: %w", err)
	}

	arrived, err := h.waitForInbound(ctx, t.marker)
	if err != nil {
		return err
	}

	if arrived.AuthResult != "pass" {
		return fmt.Errorf("sender not verified: auth result is %q", arrived.AuthResult)
	}

	current, err := h.Product.Timeline(ctx, arrived.ConversationID)
	if err != nil {
		return fmt.Errorf("read conversation: %w", err)
	}

	t.conversation, t.contact = arrived.ConversationID, current.ContactID

	return nil
}

func (h *harness) reply(ctx context.Context, t *thread) error {
	t.replyMarker = t.marker + "-reply"

	if err := h.Product.Reply(ctx, t.conversation, "Thanks for writing. "+t.replyMarker); err != nil {
		return fmt.Errorf("send reply: %w", err)
	}

	message, err := h.waitForDelivery(ctx, t.box, t.replyMarker)
	if err != nil {
		return err
	}

	if len(message.Headers["Message-Id"]) > 0 {
		t.messageID = message.Headers["Message-Id"][0]
	}

	return inspectDelivery(message)
}

func (h *harness) answer(ctx context.Context, t *thread) error {
	answer := t.marker + "-answer"
	started := h.Clock.Now()

	err := t.box.Out.Send(ctx, mail.Message{
		From:    mail.Address{Email: t.box.Address},
		To:      []mail.Address{{Email: h.Forwarding}},
		Subject: "Re: " + t.subject,
		Text:    answer + "\n\n> Thanks for writing. " + t.replyMarker,
		Headers: threadHeaders(t.messageID),
	})
	if err != nil {
		return fmt.Errorf("send answer: %w", err)
	}

	current, err := h.waitForBody(ctx, t.conversation, answer)
	if err != nil {
		return err
	}

	if took := h.Clock.Since(started); took > h.AnswerBar {
		return fmt.Errorf("answer took %s, over the bar of %s", took, h.AnswerBar)
	}

	if current.ContactID != t.contact {
		return errors.New("answer moved the conversation to a new contact")
	}

	last := current.Bodies[len(current.Bodies)-1]
	if strings.Contains(last, t.replyMarker) {
		return errors.New("quoted text was not stripped")
	}

	return nil
}

func (h *harness) replyAll(ctx context.Context, t *thread) error {
	marker := t.marker + "-cc"

	if err := h.Product.Reply(ctx, t.conversation, "Copying everyone. "+marker); err != nil {
		return fmt.Errorf("send reply: %w", err)
	}

	if _, err := h.waitForDelivery(ctx, t.box, marker); err != nil {
		return fmt.Errorf("to mailbox: %w", err)
	}

	copied, err := h.waitForDelivery(ctx, t.peer, marker)
	if err != nil {
		return fmt.Errorf("cc mailbox: %w", err)
	}

	var id string
	if len(copied.Headers["Message-Id"]) > 0 {
		id = copied.Headers["Message-Id"][0]
	}

	answer := marker + "-answer"

	err = t.peer.Out.Send(ctx, mail.Message{
		From:    mail.Address{Email: t.peer.Address},
		To:      []mail.Address{{Email: h.Forwarding}},
		Cc:      []mail.Address{{Email: t.box.Address}},
		Subject: "Re: " + t.subject,
		Text:    answer,
		Headers: threadHeaders(id),
	})
	if err != nil {
		return fmt.Errorf("send reply all: %w", err)
	}

	_, err = h.waitForBody(ctx, t.conversation, answer)

	return err
}

func threadHeaders(messageID string) map[string]string {
	if messageID == "" {
		return nil
	}

	return map[string]string{"In-Reply-To": messageID, "References": messageID}
}

/*
inspectDelivery holds what every host must show of a delivered reply.
*/
func inspectDelivery(message received) error {
	if message.Folder != folderInbox {
		return fmt.Errorf("%w: filed in %s", errSpamFolder, message.Folder)
	}

	results := strings.ToLower(strings.Join(message.Headers["Authentication-Results"], " "))
	for _, check := range []string{"spf=pass", "dkim=pass", "dmarc=pass"} {
		if !strings.Contains(results, check) {
			return fmt.Errorf("authentication results lack %s", check)
		}
	}

	if len(message.Headers["In-Reply-To"]) == 0 {
		return errors.New("reply carries no In-Reply-To header")
	}

	if slices.ContainsFunc([]string{"awstrack.me", "open.aws", "<img width=\"1\""}, func(mark string) bool {
		return strings.Contains(message.HTML, mark)
	}) {
		return errors.New("reply carries tracking")
	}

	return nil
}

/*
errSpamFolder marks a delivery the host filed outside the inbox.
*/
var errSpamFolder = errors.New("delivered to spam")

const folderInbox = "inbox"

func (h *harness) waitForInbound(ctx context.Context, marker string) (inboundMail, error) {
	var arrived inboundMail

	err := h.poll(ctx, h.Wait, "inbound mail "+marker, func() (bool, error) {
		found, err := h.Product.Inbound(ctx, h.AddressID, marker)
		arrived = found

		return found.Found, err
	})

	return arrived, err
}

func (h *harness) waitForDelivery(ctx context.Context, box mailbox, marker string) (received, error) {
	var message received

	err := h.poll(ctx, h.Wait, "delivery of "+marker+" to "+box.Host, func() (bool, error) {
		found, err := box.In.Find(ctx, marker)
		message = found

		return found.Found, err
	})

	return message, err
}

func (h *harness) waitForBody(ctx context.Context, conversation, marker string) (timeline, error) {
	var current timeline

	err := h.poll(ctx, h.Wait, marker+" in the conversation", func() (bool, error) {
		found, err := h.Product.Timeline(ctx, conversation)
		current = found

		return slices.ContainsFunc(found.Bodies, func(body string) bool {
			return strings.Contains(body, marker)
		}), err
	})

	return current, err
}

/*
poll asks until the answer is yes, the wait runs out or the context ends.
*/
func (h *harness) poll(ctx context.Context, wait time.Duration, what string, ask func() (bool, error)) error {
	deadline := h.Clock.Now().Add(wait)

	for {
		done, err := ask()
		if err != nil {
			return err
		}

		if done {
			return nil
		}

		if h.Clock.Now().After(deadline) {
			return fmt.Errorf("gave up waiting for %s after %s", what, wait)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for %s: %w", what, ctx.Err())
		case <-h.Clock.After(h.Poll):
		}
	}
}
