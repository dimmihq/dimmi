/*
|--------------------------------------------------------------------------
| Graph Subscriptions
|--------------------------------------------------------------------------
|
| One subscription per folder announces new mail. They last under seven
| days, so renewal runs daily. A subscription Microsoft no longer knows is
| created again, and one that already exists is kept. Renewing also keeps
| the reauthorization Microsoft asks for from lapsing.
|
*/

package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
watched is the state of a watch: the subscription of each folder.
*/
type watched struct {
	Received subscriptionState `json:"received"`
	Sent     subscriptionState `json:"sent"`
}

type subscriptionState struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}

/*
Watch starts or renews a subscription on each folder.
*/
func (a *Account) Watch(ctx context.Context, s mailbox.Session, in mailbox.WatchInput) (mailbox.Watching, error) {
	var existing watched
	if len(in.Existing) > 0 {
		if err := json.Unmarshal(in.Existing, &existing); err != nil {
			return mailbox.Watching{}, fmt.Errorf("graph watch state: %w", err)
		}
	}

	received, err := a.subscribe(ctx, s, in, feed{a.opts.ReceivedFolder, existing.Received.ID})
	if err != nil {
		return mailbox.Watching{}, err
	}

	sent, err := a.subscribe(ctx, s, in, feed{sentFolder, existing.Sent.ID})
	if err != nil {
		return mailbox.Watching{}, err
	}

	state, err := json.Marshal(watched{Received: received, Sent: sent})
	if err != nil {
		return mailbox.Watching{}, fmt.Errorf("graph watch state: %w", err)
	}

	expires := received.ExpiresAt
	if sent.ExpiresAt.Before(expires) {
		expires = sent.ExpiresAt
	}

	return mailbox.Watching{State: state, ExpiresAt: expires}, nil
}

/*
feed names a folder to watch and the subscription already on it, empty
for none.
*/
type feed struct {
	folder string
	id     string
}

func resource(address, folder string) string {
	return "users/" + url.PathEscape(address) + "/mailFolders/" + folder + "/messages"
}

func (a *Account) subscribe(
	ctx context.Context, s mailbox.Session, in mailbox.WatchInput, target feed,
) (subscriptionState, error) {
	expires := a.clock.Now().Add(subscription).UTC()

	if target.id != "" {
		state, err := a.renew(ctx, s, target.id, expires)
		if !errors.Is(err, errSubscriptionGone) {
			return state, err
		}
	}

	resp, err := a.api(s).Accept("application/json").Post(ctx, a.opts.BaseURL+"/subscriptions", map[string]string{
		"changeType":               "created",
		"notificationUrl":          in.NotificationURL,
		"lifecycleNotificationUrl": in.LifecycleURL,
		"resource":                 resource(s.Mailbox, target.folder),
		"expirationDateTime":       expires.Format(time.RFC3339),
		"clientState":              in.ClientState,
	})
	if err != nil {
		return subscriptionState{}, fmt.Errorf("graph subscribe: %w", err)
	}

	if resp.Status == http.StatusConflict {
		return subscriptionState{ExpiresAt: expires}, nil
	}

	if err := check(resp); err != nil {
		return subscriptionState{}, err
	}

	return decodeSubscription(resp, expires)
}

var errSubscriptionGone = errors.New("graph: subscription gone")

func (a *Account) renew(
	ctx context.Context, s mailbox.Session, id string, expires time.Time,
) (subscriptionState, error) {
	resp, err := a.api(s).Accept("application/json").Patch(ctx, a.opts.BaseURL+"/subscriptions/"+url.PathEscape(id),
		map[string]string{"expirationDateTime": expires.Format(time.RFC3339)})
	if err != nil {
		return subscriptionState{}, fmt.Errorf("graph renew: %w", err)
	}

	if resp.Status == http.StatusNotFound {
		return subscriptionState{}, errSubscriptionGone
	}

	if err := check(resp); err != nil {
		return subscriptionState{}, err
	}

	return subscriptionState{ID: id, ExpiresAt: expires}, nil
}

func decodeSubscription(resp *httpclient.Response, expires time.Time) (subscriptionState, error) {
	var created struct {
		ID string `json:"id"`
	}
	if err := resp.JSON(&created); err != nil {
		return subscriptionState{}, err
	}

	return subscriptionState{ID: created.ID, ExpiresAt: expires}, nil
}

/*
StopWatch deletes the subscriptions. One already gone is no failure.
*/
func (a *Account) StopWatch(ctx context.Context, s mailbox.Session, state []byte) error {
	var existing watched
	if err := json.Unmarshal(state, &existing); err != nil {
		return fmt.Errorf("graph watch state: %w", err)
	}

	for _, id := range []string{existing.Received.ID, existing.Sent.ID} {
		if id == "" {
			continue
		}

		resp, err := a.api(s).Delete(ctx, a.opts.BaseURL+"/subscriptions/"+url.PathEscape(id))
		if err != nil {
			return fmt.Errorf("graph stop: %w", err)
		}

		if resp.Status != http.StatusNotFound {
			if err := check(resp); err != nil {
				return err
			}
		}
	}

	return nil
}
