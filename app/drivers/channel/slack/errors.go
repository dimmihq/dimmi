/*
|--------------------------------------------------------------------------
| Slack Errors
|--------------------------------------------------------------------------
|
| Every error Slack answers with becomes a code the product keeps and a
| sentence a teammate can act on. A revoked or invalid token breaks the
| connection, so it goes to error and the admins hear of it once. A bot
| missing from a channel, or a channel gone, fails that message for good
| with the fix in its reason. Slack asking to slow down postpones the
| send by the time it names, without spending an attempt. Slack's own
| outage and the network are transient and retried.
|
*/

package slack

import (
	"errors"
	"fmt"

	"github.com/slack-go/slack"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
The codes of Slack's errors the product acts on.
*/
const (
	CodeAuthFailed      = "slack_auth_failed"
	CodeNotInChannel    = "slack_not_in_channel"
	CodeChannelGone     = "slack_channel_gone"
	CodeContentRejected = "slack_content_rejected"
	CodeRestricted      = "slack_restricted"
	CodeThreadUnknown   = "slack_thread_unknown"
	CodeAlreadyReacted  = "slack_already_reacted"
	CodeNoReaction      = "slack_no_reaction"
	CodeMessageNotFound = "slack_message_not_found"
)

/*
errorClass is what one of Slack's error codes means: the product's code,
the sentence shown, and the sentinel it wraps.
*/
type errorClass struct {
	code     string
	message  string
	sentinel error
}

/*
classOf returns what Slack's error code means.
*/
func classOf(slackCode string) errorClass {
	switch slackCode {
	case "invalid_auth", "account_inactive", "token_revoked", "token_expired", "not_authed":
		return errorClass{
			CodeAuthFailed, "Slack no longer accepts the app's token. Reconnect Slack.",
			contracts.ErrConnectionBroken,
		}
	case "not_in_channel":
		return errorClass{
			CodeNotInChannel, "The app is not in this Slack channel. Invite it to the channel.",
			contracts.ErrPermanent,
		}
	case "channel_not_found", "is_archived":
		return errorClass{CodeChannelGone, "The Slack channel was archived or deleted.", contracts.ErrPermanent}
	case "msg_too_long", "no_text", "invalid_blocks", "markdown_text_conflict":
		return errorClass{CodeContentRejected, "Slack refused the message's content.", contracts.ErrContentRejected}
	case "ekm_access_denied", "team_access_not_granted", "restricted_action":
		return errorClass{
			CodeRestricted, "A Slack admin policy blocks the app from posting here.",
			contracts.ErrPermanent,
		}
	default:
		return errorClass{
			"slack_" + slackCode, "Slack refused the request (" + slackCode + ").", contracts.ErrPermanent,
		}
	}
}

/*
MapError turns an error from the Slack SDK into the channel contract's
errors: a rate limit to RateLimitedError, Slack's answers to a
SendError with the product's code, and anything else to a transient
error.
*/
func MapError(method string, err error) error {
	if err == nil {
		return nil
	}

	if limited, ok := errors.AsType[*slack.RateLimitedError](err); ok {
		return &contracts.RateLimitedError{RetryAfter: limited.RetryAfter}
	}

	if answer, ok := errors.AsType[slack.SlackErrorResponse](err); ok {
		class := classOf(answer.Err)

		return &contracts.SendError{Code: class.code, Message: class.message, Err: class.sentinel}
	}

	return fmt.Errorf("slack %s: %w", method, err)
}

/*
Code returns the product's code of a mapped error, or empty.
*/
func Code(err error) string {
	if sendErr, ok := errors.AsType[*contracts.SendError](err); ok {
		return sendErr.Code
	}

	return ""
}
