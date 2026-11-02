/*
|--------------------------------------------------------------------------
| SNS Messages
|--------------------------------------------------------------------------
|
| SNS posts JSON documents to webhook endpoints: notifications, and
| requests to confirm or cancel a subscription. Anyone can post to a
| public endpoint, so a message is only data until its signature checks
| out.
|
*/

package sns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

/*
Message types SNS posts.
*/
const (
	TypeNotification             = "Notification"
	TypeSubscriptionConfirmation = "SubscriptionConfirmation"
	TypeUnsubscribeConfirmation  = "UnsubscribeConfirmation"
)

/*
ErrInvalidMessage rejects a body that is not an SNS message.
*/
var ErrInvalidMessage = errors.New("sns: invalid message")

/*
Message is one SNS message with the fields SNS signs. Subject is set on
notifications only when the publisher gave one; Token and SubscribeURL
only on subscription messages.
*/
type Message struct {
	Type             string
	MessageID        string
	TopicARN         string
	Subject          string
	Message          string
	Timestamp        string
	Token            string
	SubscribeURL     string
	UnsubscribeURL   string
	SignatureVersion string
	Signature        string
	SigningCertURL   string
}

/*
Parse reads a message from the posted body. It checks the shape only;
Verify checks that SNS sent it.
*/
func Parse(body []byte) (Message, error) {
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		return Message{}, fmt.Errorf("%w: %w", ErrInvalidMessage, err)
	}

	text := func(key string) string {
		value, _ := fields[key].(string)

		return value
	}

	msg := Message{
		Type:             text("Type"),
		MessageID:        text("MessageId"),
		TopicARN:         text("TopicArn"),
		Subject:          text("Subject"),
		Message:          text("Message"),
		Timestamp:        text("Timestamp"),
		Token:            text("Token"),
		SubscribeURL:     text("SubscribeURL"),
		UnsubscribeURL:   text("UnsubscribeURL"),
		SignatureVersion: text("SignatureVersion"),
		Signature:        text("Signature"),
		SigningCertURL:   text("SigningCertURL"),
	}

	if msg.Type == "" || msg.MessageID == "" || msg.TopicARN == "" || msg.Signature == "" {
		return Message{}, fmt.Errorf("%w: missing type, id, topic or signature", ErrInvalidMessage)
	}

	return msg, nil
}

/*
signedText is the text SNS signs: chosen fields, each as its name and
value on their own lines, in a fixed order per message type.
*/
func signedText(msg Message) (string, error) {
	var pairs [][2]string

	switch msg.Type {
	case TypeNotification:
		pairs = [][2]string{{"Message", msg.Message}, {"MessageId", msg.MessageID}}
		if msg.Subject != "" {
			pairs = append(pairs, [2]string{"Subject", msg.Subject})
		}

		pairs = append(pairs, [2]string{"Timestamp", msg.Timestamp}, [2]string{"TopicArn", msg.TopicARN})
	case TypeSubscriptionConfirmation, TypeUnsubscribeConfirmation:
		pairs = [][2]string{
			{"Message", msg.Message},
			{"MessageId", msg.MessageID},
			{"SubscribeURL", msg.SubscribeURL},
			{"Timestamp", msg.Timestamp},
			{"Token", msg.Token},
			{"TopicArn", msg.TopicARN},
		}
	default:
		return "", fmt.Errorf("%w: unknown type %q", ErrInvalidMessage, msg.Type)
	}

	pairs = append(pairs, [2]string{"Type", msg.Type})

	var text strings.Builder
	for _, pair := range pairs {
		text.WriteString(pair[0] + "\n" + pair[1] + "\n")
	}

	return text.String(), nil
}
