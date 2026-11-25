/*
|--------------------------------------------------------------------------
| What A Procedure Reads
|--------------------------------------------------------------------------
|
| The facts a procedure's rules decide on, read from the customer's
| trusted fields alone; the names its chips show; and the help articles
| it cites, loaded as sources the agent may cite when the reader's
| audiences allow.
|
*/

package pipeline

import (
	"context"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
procedureFacts is how the procedure's rules read a field: the
customer's own fields and only their trusted custom attributes, their
company's, and the conversation's channel, inbox and whether the
customer is signed in.
*/
func (r *Runner) procedureFacts(ctx context.Context, st *run) func(string) (any, bool) {
	e := st.env
	if st.sim != nil {
		return st.sim.lookup
	}
	var person *audiences.Person
	if e.Contact != nil {
		_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			people, err := audiences.LoadPeople(ctx, models.New(tx), e.Workspace.ID, []models.Contact{*e.Contact})
			if err == nil && len(people) == 1 {
				p := people[0]
				p.TrustedOnly, p.Now = true, r.Clock.Now()
				person = &p
			}
			return err
		})
	}
	verified := st.actions.identity.Verified
	return func(field string) (any, bool) {
		switch field {
		case "conversation.channel":
			return e.ChannelType, true
		case "conversation.inbox_id":
			return models.PublicID(models.PrefixInbox, e.Conversation.InboxID), true
		case "conversation.verified":
			return verified, true
		}
		if person == nil {
			return nil, false
		}
		return person.Lookup(factField(field))
	}
}

/*
factField maps a rule's field to the name audiences read: attr becomes
custom.
*/
func factField(field string) string {
	for _, root := range []string{"contact.", "company."} {
		if rest, ok := strings.CutPrefix(field, root+"attr."); ok {
			return root + "custom." + rest
		}
	}
	return field
}

/*
procedureLabels names what chips point at: teams, tags, ticket types
and procedures by public id.
*/
func procedureLabels(e *env) map[string]string {
	out := map[string]string{}
	for _, t := range e.Triage.workspaceTeams {
		out[procedure.BindHandoff+":"+models.PublicID(models.PrefixTeam, t.ID)] = t.Name
	}
	for _, t := range e.Triage.workspaceTags {
		out[procedure.BindTag+":"+models.PublicID(models.PrefixTag, t.ID)] = t.Name
	}
	for _, t := range e.TicketTypes {
		out[procedure.BindTicketType+":"+models.PublicID(models.PrefixTicketType, t.ID)] = t.Name
	}
	for _, p := range e.AllProcedures {
		out[procedure.BindSubProcedure+":"+p.publicID()] = p.row.Name
	}
	return out
}

/*
preloadArticles gives the model every article the procedure cites as a
numbered source, the reader's audiences permitting, as soon as it is
followed.
*/
func (r *Runner) preloadArticles(ctx context.Context, st *run) {
	if r.Knowledge == nil {
		return
	}
	refs := procedure.References(st.proc.live.def).Articles
	if len(refs) == 0 {
		return
	}
	allowed, filtered, err := r.readerAudiences(ctx, st)
	if err != nil {
		return
	}
	var restrictions audiences.Restrictions
	if filtered {
		_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			restrictions, err = audiences.LoadRestrictions(ctx, models.New(tx), st.env.Workspace.ID)
			return err
		})
	}
	for _, public := range refs {
		id, ok := decodePublic(models.PrefixArticle, public)
		if !ok || (filtered && !restrictions.Article(id, allowed)) {
			continue
		}
		doc, err := r.Knowledge.Document(ctx, st.env.Workspace.ID, id, []contracts.KnowledgeVisibility{
			contracts.KnowledgeVisibilityPublic, contracts.KnowledgeVisibilityAgentOnly,
		})
		if err != nil {
			continue
		}
		n := st.sourceFor(contracts.KnowledgeChunk{
			ID: doc.ID, DocumentID: doc.ID, ArticleID: &id, Title: doc.Title, URL: doc.URL,
			Text: truncate(doc.Text, carriedChars), MatchedBy: []string{"procedure"}, Visibility: doc.Visibility,
		})
		st.proc.labels[procedure.BindArticle+":"+public] = doc.Title
		st.proc.articles = append(st.proc.articles, "["+strconv.Itoa(n)+"] "+doc.Title+": "+
			truncate(doc.Text, carriedChars))
	}
}
