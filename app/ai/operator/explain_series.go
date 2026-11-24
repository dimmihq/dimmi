/*
|--------------------------------------------------------------------------
| Explaining Series Journeys
|--------------------------------------------------------------------------
|
| Operator explains what series did with a person from what was
| recorded: each journey, the version it follows, every step it reached
| with the path it took and what came of it, what it waits for now, the
| messages it sent and how they were read, and how it ended. For each
| series it also shows how the live entry and exit rules judge the
| contact right now, condition by condition, so it can say why someone
| never entered. Every step is named by its id, kind and what it says,
| so the answer needs nothing else.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
stamp is how times read in explanations.
*/
const stamp = "2006-01-02 15:04:05 UTC"

/*
explainSeriesTool is the tool that explains a person's series journeys.
*/
func (e *Engine) explainSeriesTool() tool {
	return tool{spec: contracts.ToolSpec{
		Name: "explain_series_journey",
		Description: "Explain what series did with a person: whether and when they entered each series and how " +
			"(rules, saved audience, event or by hand), every step their journey reached with the path it took " +
			"and its result, what it waits for now and until when, the messages it sent with opens, clicks and " +
			"replies, the goal, and how it ended. For each live series it also tests the entry and exit rules " +
			"on the contact now, condition by condition, to say why they did not enter.",
		Schema: schema(`{"type":"object","properties":{` +
			`"contact_id":{"type":"string","description":"The contact (ctc_...)."},` +
			`"email":{"type":"string","description":"The contact's email, instead of contact_id."},` +
			`"enrollment_id":{"type":"string","description":"One journey (sren_...) instead of a contact."},` +
			`"series_id":{"type":"string","description":"Only this series (srs_...); leave out for every ` +
			`series the person traveled and every live one."}}}`),
	}, handle: e.explainSeriesJourney, was: map[string]string{
		"contact": "contact_id", "enrollment": "enrollment_id", "journey_id": "enrollment_id",
		"series": "series_id",
	}}
}

/*
explainSeriesJourney explains a person's journeys.
*/
func (e *Engine) explainSeriesJourney(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Contact    string `json:"contact_id"`
		Email      string `json:"email"`
		Enrollment string `json:"enrollment_id"`
		Series     string `json:"series_id"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	ask := series.ExplainInput{Membership: s.m, Email: strings.TrimSpace(in.Email)}
	for _, arg := range []struct {
		value  string
		prefix models.Prefix
		into   **ids.UUID
	}{
		{in.Contact, models.PrefixContact, &ask.ContactID},
		{in.Enrollment, models.PrefixSeriesEnrollment, &ask.EnrollmentID},
		{in.Series, models.PrefixSeries, &ask.SeriesID},
	} {
		if strings.TrimSpace(arg.value) == "" {
			continue
		}
		id, err := e.publicID(arg.value, arg.prefix)
		if err != nil {
			return result{}, fmt.Errorf("%w: %q is not a %s_... id", errBadArguments, arg.value, arg.prefix)
		}
		*arg.into = &id
	}
	if ask.ContactID == nil && ask.Email == "" && ask.EnrollmentID == nil {
		return result{}, fmt.Errorf("%w: name the person by contact_id or email, or a journey by enrollment_id",
			errBadArguments)
	}
	got, err := e.Services.Series.Explain(ctx, ask)
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	return result{text: truncate(e.journeysText(s, got), 40000), step: Step{
		Tool: "explain_series_journey", Summary: "Read what series did with a person",
	}}, nil
}

/*
journeysText writes a person's series: who they are, then each series
with its journeys and how its rules judge them now.
*/
func (e *Engine) journeysText(s *session, x series.Explanation) string {
	var b strings.Builder
	if x.Contact != nil {
		writeContact(&b, *x.Contact)
	}
	if len(x.Series) == 0 {
		b.WriteString("\nThis person never traveled a series, and the workspace has no live series.\n")
	}
	for _, story := range x.Series {
		b.WriteString("\n")
		e.writeStory(&b, s, story)
	}
	return b.String()
}

/*
writeContact writes who the person is.
*/
func writeContact(b *strings.Builder, c models.Contact) {
	fmt.Fprintf(b, "Contact %s", deref(c.Name))
	if c.Email != nil {
		fmt.Fprintf(b, " <%s>", *c.Email)
	}
	fmt.Fprintf(b, " · contact_id %s", models.PublicID(models.PrefixContact, c.ID))
	if c.CompanyID != nil {
		fmt.Fprintf(b, " · company_id %s", models.PublicID(models.PrefixCompany, *c.CompanyID))
	} else {
		b.WriteString(" · no company")
	}
	if c.BlockedAt != nil {
		b.WriteString(" · blocked, so no series reaches them")
	}
	b.WriteString("\n")
}

/*
writeStory writes one series as it concerns the person.
*/
func (e *Engine) writeStory(b *strings.Builder, s *session, story series.Story) {
	row := story.Series
	fmt.Fprintf(b, "Series %q · series_id %s · for %ss · %s · link %s\n", row.Name,
		models.PublicID(models.PrefixSeries, row.ID), row.Subject, row.Status,
		e.link(s, linkSeries, models.PrefixSeries, row.ID))
	if story.Live == nil {
		b.WriteString("  Never published, so nobody has entered it.\n")
	} else {
		fmt.Fprintf(b, "  Live version %d (%s), published %s\n", story.Live.Number,
			models.PublicID(models.PrefixSeriesVersion, story.Live.ID), story.Live.CreatedAt.UTC().Format(stamp))
		writeRules(b, *story.LiveSpec)
	}
	if len(story.Journeys) == 0 {
		b.WriteString("  Journeys: none. This person never entered this series.\n")
	} else {
		b.WriteString("  Journeys, newest first:\n")
	}
	for _, j := range story.Journeys {
		e.writeJourney(b, s, j)
	}
	if story.Entry != nil {
		writeEntryCheck(b, *story.Entry)
	}
}

/*
writeJourney writes one journey: how it entered, where it stands, every
step it reached and every message it sent.
*/
func (e *Engine) writeJourney(b *strings.Builder, s *session, j series.JourneyStory) {
	en := j.Enrollment
	writeEntered(b, j)
	if en.EndReason != nil && *en.EndReason == "already_matched" {
		b.WriteString("    Kept out: they already matched when the series went live, and it lets only people " +
			"who start matching later enter.\n")
		return
	}
	v := newTraveled(j.Spec)
	writeStanding(b, v, en)
	b.WriteString("    Steps reached, in order:\n")
	if len(j.Steps) == 0 {
		b.WriteString("      none yet\n")
	}
	for i, st := range j.Steps {
		v.writeStep(b, i+1, st)
	}
	if len(j.Messages) > 0 {
		b.WriteString("    Messages:\n")
	}
	for _, m := range j.Messages {
		e.writeMessage(b, s, v.nodes, m)
	}
}

/*
writeEntered writes how and when a journey entered, and the version it
follows.
*/
func writeEntered(b *strings.Builder, j series.JourneyStory) {
	en := j.Enrollment
	fmt.Fprintf(b, "  - journey enrollment_id %s · status %s · entered %s by %s", models.PublicID(
		models.PrefixSeriesEnrollment, en.ID), en.Status, en.EnteredAt.UTC().Format(stamp), en.EnteredBy)
	if en.TriggerRef != nil {
		fmt.Fprintf(b, " (%s)", *en.TriggerRef)
	}
	fmt.Fprintf(b, " · follows version %d (%s)", j.Version.Number,
		models.PublicID(models.PrefixSeriesVersion, j.Version.ID))
	if en.Subject == series.SubjectCompany && en.CompanyID != nil {
		fmt.Fprintf(b, " · for the company %s", models.PublicID(models.PrefixCompany, *en.CompanyID))
	}
	if en.Test {
		b.WriteString(" · a test journey (Try it on me), which never counts in the report")
	}
	b.WriteString("\n")
}

/*
writeStanding writes where a journey stands: the step it is at and what
it waits for, or how it ended; the goal; and the split groups drawn.
*/
func writeStanding(b *strings.Builder, v traveled, en models.ListPersonSeriesJourneysRow) {
	nodes := v.nodes
	if en.LeftAt == nil {
		fmt.Fprintf(b, "    Now at step %s\n", stepName(nodes, en.NodeID))
		writeWait(b, en)
	} else {
		fmt.Fprintf(b, "    Ended %s · reason %s · last step %s\n", en.LeftAt.UTC().Format(stamp),
			deref(en.EndReason), stepName(nodes, en.NodeID))
	}
	if en.GoalAt != nil {
		fmt.Fprintf(b, "    Reached the goal %s\n", en.GoalAt.UTC().Format(stamp))
	}
	if drawn := strings.TrimSpace(string(en.Variants)); drawn != "" && drawn != "{}" && drawn != "null" {
		fmt.Fprintf(b, "    Split groups drawn (split step id: group): %s\n", truncate(drawn, 500))
	}
}

/*
writeWait writes what a journey waits for now and until when.
*/
func writeWait(b *strings.Builder, en models.ListPersonSeriesJourneysRow) {
	var w struct {
		Kind   string    `json:"kind"`
		Until  time.Time `json:"until"`
		Result string    `json:"result"`
	}
	if len(en.Wait) == 0 || json.Unmarshal(en.Wait, &w) != nil || w.Kind == "" {
		if en.WakeAt != nil {
			fmt.Fprintf(b, "    Moves on next at %s\n", en.WakeAt.UTC().Format(stamp))
		}
		return
	}
	what := map[string]string{
		"timer":   "a wait step's time to pass",
		"match":   "the person to do or match what the step waits for, or its time limit",
		"send":    "the message to finish sending",
		"cooloff": "quiet hours or the daily message cap before the message may go",
		"call":    "an outside call to answer",
	}[w.Kind]
	if what == "" {
		what = w.Kind
	}
	fmt.Fprintf(b, "    Waiting for %s (wait kind %s)", what, w.Kind)
	switch {
	case !w.Until.IsZero():
		fmt.Fprintf(b, " until %s", w.Until.UTC().Format(stamp))
	case en.WakeAt != nil:
		fmt.Fprintf(b, " until %s", en.WakeAt.UTC().Format(stamp))
	}
	if w.Result != "" {
		fmt.Fprintf(b, " · because %s", w.Result)
	}
	b.WriteString("\n")
}

/*
traveled is the version a journey follows: its steps by id and its
paths.
*/
type traveled struct {
	nodes map[string]series.Node
	edges []series.Edge
}

/*
newTraveled indexes a version's steps by id.
*/
func newTraveled(spec series.Spec) traveled {
	nodes := make(map[string]series.Node, len(spec.Nodes))
	for _, n := range spec.Nodes {
		nodes[n.ID] = n
	}
	return traveled{nodes: nodes, edges: spec.Edges}
}

/*
writeStep writes one step a journey reached.
*/
func (v traveled) writeStep(b *strings.Builder, at int, st models.SeriesStep) {
	edges := v.edges
	fmt.Fprintf(b, "      %d. step %s · entered %s · %s", at, stepName(v.nodes, st.NodeID),
		st.EnteredAt.UTC().Format(stamp), st.Status)
	if st.Outcome != nil {
		fmt.Fprintf(b, " · result %s", *st.Outcome)
	}
	if st.PathID != nil {
		fmt.Fprintf(b, " · took path %s", *st.PathID)
		if to, ok := edgeTo(edges, st.NodeID, *st.PathID); ok {
			fmt.Fprintf(b, " → %s", to)
		} else {
			b.WriteString(" → nothing, so the journey ended there")
		}
	}
	if st.LeftAt != nil {
		fmt.Fprintf(b, " · left %s", st.LeftAt.UTC().Format(stamp))
	}
	if d := strings.TrimSpace(string(st.Detail)); d != "" && d != "{}" && d != "null" {
		fmt.Fprintf(b, " · detail %s", truncate(compactJSON(st.Detail), 400))
	}
	b.WriteString("\n")
}

/*
writeMessage writes one message a journey sent or tried to send, with
how it was read.
*/
func (e *Engine) writeMessage(b *strings.Builder, s *session, nodes map[string]series.Node,
	m models.ListSeriesEnrollmentMessagesRow,
) {
	fmt.Fprintf(b, "      - from step %s", stepName(nodes, m.NodeID))
	if m.Address != "" {
		fmt.Fprintf(b, " to %s", m.Address)
	}
	fmt.Fprintf(b, " · %s", m.Status)
	if m.Reason != "" {
		fmt.Fprintf(b, " (%s)", m.Reason)
	}
	for _, at := range []struct {
		what string
		when *time.Time
	}{
		{"sent", m.SentAt},
		{"delivered", m.DeliveredAt},
		{"opened", m.OpenedAt},
		{"clicked", m.ClickedAt},
		{"replied", m.RepliedAt},
		{"unsubscribed", m.UnsubscribedAt},
	} {
		if at.when != nil {
			fmt.Fprintf(b, " · %s %s", at.what, at.when.UTC().Format(stamp))
		}
	}
	if m.SentAt == nil && m.Status == "queued" {
		fmt.Fprintf(b, " · due %s", m.DueAt.UTC().Format(stamp))
	}
	if m.ConversationID != nil {
		fmt.Fprintf(b, " · conversation %s", e.conversationLink(s, *m.ConversationID))
	}
	b.WriteString("\n")
}

/*
stepName names a step by its id, kind and what it says, from the version
the journey follows.
*/
func stepName(nodes map[string]series.Node, id string) string {
	n, ok := nodes[id]
	if !ok {
		return id + " (no longer in the version this journey follows)"
	}
	name := id + " (" + n.Kind
	var settings struct {
		Subject string `json:"subject"`
		Title   string `json:"title"`
	}
	_ = json.Unmarshal(n.Settings, &settings)
	switch {
	case settings.Subject != "":
		name += fmt.Sprintf(" %q", settings.Subject)
	case settings.Title != "":
		name += fmt.Sprintf(" %q", settings.Title)
	case len(n.Settings) > 0 && !series.IsMessage(n.Kind):
		name += " " + truncate(compactJSON(n.Settings), 300)
	}
	return name + ")"
}
