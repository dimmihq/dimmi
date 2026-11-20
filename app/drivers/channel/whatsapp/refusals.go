/*
|--------------------------------------------------------------------------
| Refused Templates
|--------------------------------------------------------------------------
|
| A template dimmi will not send, or one Meta refused, comes back to the
| teammate with its reason: missing in that language, paused, disabled,
| rejected, waiting for review, changed since it was picked, too long once
| filled in, or a variable in the wrong form. When Meta refuses one, dimmi
| asks Meta for the connection's templates again first, so the reason is
| what Meta says now and the composer offers only what still sends.
|
*/

package whatsapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Meta's codes for a refused template.
*/
const (
	metaParamsMismatch = 132000
	metaNoTranslation  = 132001
	metaTooLong        = 132005
	metaPolicy         = 132007
	metaParamFormat    = 132012
	metaPaused         = 132015
	metaDisabled       = 132016
)

/*
templateCode reports whether Meta's code is about the template.
*/
func templateCode(code int) bool {
	return code >= 132000 && code < 133000
}

/*
refused asks Meta for the connection's templates again and names why the
template did not go out, by what Meta says of it now.
*/
func (d *Driver) refused(ctx context.Context, s sending, choice Choice, code int) error {
	if err := d.Places.RefreshTemplates(ctx, s.conn); err != nil {
		return templateRefusal(code, choice, &Template{})
	}

	t, ok, err := d.Places.Template(ctx, s.conn, choice.Name, choice.Language)
	if err != nil {
		return templateRefusal(code, choice, &Template{})
	}

	if !ok {
		return templateRefusal(code, choice, nil)
	}

	return templateRefusal(code, choice, &t)
}

/*
templateRefusal is the failed send for a template: found is the
connection's copy, nil when it has none; code is Meta's refusal, zero
when dimmi held it back itself.
*/
func templateRefusal(code int, choice Choice, found *Template) *contracts.SendError {
	return &contracts.SendError{
		Code: CodeTemplateRejected, Message: RefusalReason(code, choice, found), Err: contracts.ErrContentRejected,
	}
}

/*
RefusalReason words why the template did not go out. What Meta says of
the template now comes first; Meta's code explains the rest.
*/
func RefusalReason(code int, choice Choice, found *Template) string {
	name := choice.Name

	if found == nil || code == metaNoTranslation {
		return fmt.Sprintf("Meta has no template %s in %s. Pick another with /.", name, choice.Language)
	}

	if found.Status != "" {
		if reason, held := statusReason(name, *found); held {
			return reason
		}
	}

	return codeReason(code, name, *found)
}

/*
statusReason words a template Meta will not send as it stands, by its
status.
*/
func statusReason(name string, t Template) (string, bool) {
	switch strings.ToUpper(t.Status) {
	case "APPROVED":
		if t.Sendable() {
			return "", false
		}

		return fmt.Sprintf("dimmi cannot send %s yet: it has a media header, a button that needs a value "+
			"or a card dimmi does not fill in.", name), true
	case "PAUSED":
		return fmt.Sprintf("Meta paused %s because customers blocked or reported it. "+
			"It sends again once Meta lifts the pause.", name), true
	case "DISABLED":
		return fmt.Sprintf("Meta disabled %s after it stayed low quality. Create a new template.", name), true
	case "REJECTED":
		if t.RejectedReason != "" && !strings.EqualFold(t.RejectedReason, "NONE") {
			return fmt.Sprintf("Meta rejected %s (%s).", name, humanize(t.RejectedReason)), true
		}

		return fmt.Sprintf("Meta rejected %s.", name), true
	case "PENDING", "IN_APPEAL", "PENDING_DELETION":
		return fmt.Sprintf("%s is still waiting for Meta's review.", name), true
	default:
		return fmt.Sprintf("Meta has %s as %s, not approved.", name, humanize(t.Status)), true
	}
}

/*
codeReason words Meta's refusal of an approved template.
*/
func codeReason(code int, name string, t Template) string {
	switch code {
	case metaParamsMismatch:
		if t.Status == "" {
			return fmt.Sprintf("The variables sent do not match %s as Meta has it now. Pick it again with /.", name)
		}

		return fmt.Sprintf("%s changed in WhatsApp Manager since it was picked; it now takes %d %s. "+
			"Pick it again with /.", name, len(t.Variables()), plural(len(t.Variables()), "variable"))
	case metaTooLong:
		return fmt.Sprintf("Filled in, %s is longer than WhatsApp allows. Shorten its variables.", name)
	case metaPolicy:
		return fmt.Sprintf("Meta refused the filled-in %s under its content rules.", name)
	case metaParamFormat:
		return fmt.Sprintf("A variable of %s is not in the form the template expects, such as a date or amount.",
			name)
	case metaPaused:
		return fmt.Sprintf("Meta paused %s because customers blocked or reported it.", name)
	case metaDisabled:
		return fmt.Sprintf("Meta disabled %s after it stayed low quality. Create a new template.", name)
	default:
		return fmt.Sprintf("Meta refused %s (error %d).", name, code)
	}
}

/*
humanize writes one of Meta's upper-case words as a phrase.
*/
func humanize(word string) string {
	return strings.ToLower(strings.ReplaceAll(word, "_", " "))
}

/*
plural writes the noun for the count.
*/
func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}

	return noun + "s"
}
