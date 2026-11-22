/*
|--------------------------------------------------------------------------
| Operator Routine Mail
|--------------------------------------------------------------------------
|
| A teammate on a routine's email list gets what a run found: the report
| Operator wrote, the changes it suggested with what each would do, and
| a link to review them in dimmi.
|
*/

package mail

import (
	"context"
	"strconv"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
RoutineSuggestion is one change a routine suggested.
*/
type RoutineSuggestion = accounts.RoutineSuggestion

/*
OperatorRoutineMail is one run of Routine in Workspace, for Name: how it
ended (findings, noop, failed or paused), its report a paragraph at a
time, its suggestions and where to review them.
*/
type OperatorRoutineMail struct {
	Name        string              `json:"name"`
	Workspace   string              `json:"workspace"`
	Routine     string              `json:"routine"`
	Outcome     string              `json:"outcome"`
	Report      []string            `json:"report"`
	Suggestions []RoutineSuggestion `json:"suggestions"`
	ReviewURL   string              `json:"review_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (OperatorRoutineMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject saying how the run ended.
*/
func (m OperatorRoutineMail) Content(ctx context.Context) (mail.Content, error) {
	outcome := m.Outcome
	switch outcome {
	case "findings", "noop", "failed", "paused":
	default:
		outcome = "noop"
	}
	args := i18n.Args{
		"routine": m.Routine, "workspace": m.Workspace, "count": strconv.Itoa(len(m.Suggestions)),
	}
	subject := i18n.T(ctx, "mail.operator_routine.subject_"+outcome, args)
	intro := i18n.T(ctx, "mail.operator_routine.intro", args)
	c := mail.View(accounts.OperatorRoutineReport(subject, m.Name, intro, m.Report, m.Suggestions, m.ReviewURL))
	c.Subject = subject
	return c, nil
}

/*
RegisterOperatorRoutines lets routine mails be queued.
*/
func RegisterOperatorRoutines(m *mail.Manager) {
	mail.Register[OperatorRoutineMail](m, "operator_routine")
}
