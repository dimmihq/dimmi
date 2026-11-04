//go:build emailmatrix

/*
|--------------------------------------------------------------------------
| The Application Under Test
|--------------------------------------------------------------------------
|
| The harness sees the staging application the way an API customer does: its
| public REST API with a token. That keeps it honest, because nothing here
| reaches around the routes real callers use.
|
*/

package emailmatrix

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
product is what the scenarios need from the application.
*/
type product interface {
	/*
		Inbound finds mail that arrived at the address row with the marker in its
		subject. Found is false while it has not arrived.
	*/
	Inbound(ctx context.Context, addressID, marker string) (inboundMail, error)

	/*
		Reply sends a teammate's reply in the conversation.
	*/
	Reply(ctx context.Context, conversationID, body string) error

	/*
		Timeline returns the bodies of the conversation's messages and its contact.
	*/
	Timeline(ctx context.Context, conversationID string) (timeline, error)
}

/*
inboundMail is one received mail. AuthResult is pass when its sender proved
the domain.
*/
type inboundMail struct {
	Found          bool
	ConversationID string
	AuthResult     string
}

/*
timeline is what a conversation holds. Bodies is every message body, oldest
first.
*/
type timeline struct {
	ContactID string
	Bodies    []string
}

/*
apiProduct is the product over its REST API.
*/
type apiProduct struct {
	client  *httpclient.Client
	baseURL string
	token   string
}

func newAPIProduct(baseURL, token string) (*apiProduct, error) {
	client, err := httpclient.New(httpclient.Config{})
	if err != nil {
		return nil, fmt.Errorf("http client: %w", err)
	}

	return &apiProduct{client: client, baseURL: strings.TrimRight(baseURL, "/") + "/api/v1", token: token}, nil
}

func (p *apiProduct) get(ctx context.Context, path string, into any) error {
	response, err := p.client.New().WithToken(p.token).Get(ctx, p.baseURL+path)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}

	if err := response.JSON(into); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}

	return nil
}

/*
Inbound finds the mail carrying the marker that reached the address.
*/
func (p *apiProduct) Inbound(ctx context.Context, addressID, marker string) (inboundMail, error) {
	var page struct {
		Data []struct {
			Subject      *string `json:"subject"`
			Conversation *struct {
				ID string `json:"id"`
			} `json:"conversation"`
			Auth json.RawMessage `json:"auth"`
		} `json:"data"`
	}

	if err := p.get(ctx, "/email-addresses/"+addressID+"/inbound-emails", &page); err != nil {
		return inboundMail{}, err
	}

	for _, row := range page.Data {
		if row.Subject == nil || !strings.Contains(*row.Subject, marker) || row.Conversation == nil {
			continue
		}

		var auth struct {
			Result string `json:"result"`
		}

		if err := json.Unmarshal(row.Auth, &auth); err != nil {
			return inboundMail{}, fmt.Errorf("decode auth of mail %s: %w", marker, err)
		}

		return inboundMail{Found: true, ConversationID: row.Conversation.ID, AuthResult: auth.Result}, nil
	}

	return inboundMail{}, nil
}

/*
Reply writes a teammate reply on the conversation.
*/
func (p *apiProduct) Reply(ctx context.Context, conversationID, body string) error {
	path := "/conversations/" + conversationID + "/replies"

	_, err := p.client.New().WithToken(p.token).Post(ctx, p.baseURL+path, map[string]string{"body": body})
	if err != nil {
		return fmt.Errorf("post %s: %w", path, err)
	}

	return nil
}

/*
Timeline reads what the conversation holds now.
*/
func (p *apiProduct) Timeline(ctx context.Context, conversationID string) (timeline, error) {
	var conversation struct {
		Data struct {
			Contact struct {
				ID string `json:"id"`
			} `json:"contact"`
		} `json:"data"`
	}

	if err := p.get(ctx, "/conversations/"+conversationID, &conversation); err != nil {
		return timeline{}, err
	}

	var messages struct {
		Data []struct {
			Body *string `json:"body"`
		} `json:"data"`
	}

	if err := p.get(ctx, "/conversations/"+conversationID+"/messages", &messages); err != nil {
		return timeline{}, err
	}

	result := timeline{ContactID: conversation.Data.Contact.ID}

	for _, message := range messages.Data {
		if message.Body != nil {
			result.Bodies = append(result.Bodies, *message.Body)
		}
	}

	return result, nil
}
