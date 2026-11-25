/*
|--------------------------------------------------------------------------
| Explaining Series Rules
|--------------------------------------------------------------------------
|
| How an explanation writes a series' rules: who enters, what ends a
| journey early and the goal, as the live version has them, and how
| those rules judge the contact right now, condition by condition with
| the value the contact has.
|
*/

package operator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
writeRules writes who enters a version, what makes them leave and the
goal.
*/
func writeRules(b *strings.Builder, spec series.Spec) {
	writeEntry(b, spec.Entry)
	writeExit(b, spec.Exit)
	writeGoal(b, spec.Goal)
}

/*
writeEntry writes who enters and whether they may enter again.
*/
func writeEntry(b *strings.Builder, entry series.Entry) {
	var how []string
	if entry.AudienceID != "" {
		how = append(how, "being in the saved audience "+entry.AudienceID)
	}
	if len(entry.Rules) > 0 && string(entry.Rules) != "null" {
		how = append(how, "matching the rules "+truncate(compactJSON(entry.Rules), 1500))
	}
	if entry.Event != "" {
		how = append(how, "doing the event "+entry.Event)
	}
	if len(how) == 0 {
		how = append(how, "being added by hand or through the API only")
	}
	fmt.Fprintf(b, "  Entry: by %s · people who already matched when it went live enter too: %t · re-entry: %s",
		strings.Join(how, ", "), entry.IncludeExisting, entry.Reentry.Kind)
	if entry.Reentry.Kind == "again" {
		fmt.Fprintf(b, " after %d days", entry.Reentry.CooldownDays)
	}
	b.WriteString("\n")
}

/*
writeExit writes what ends a journey early.
*/
func writeExit(b *strings.Builder, exit series.Exit) {
	var leaves []string
	if len(exit.Rules) > 0 && string(exit.Rules) != "null" {
		leaves = append(leaves, "matching the rules "+truncate(compactJSON(exit.Rules), 1500))
	}
	for _, on := range []struct {
		set  bool
		what string
	}{
		{exit.OnGoal, "reaching the goal"},
		{exit.OnReply, "replying to any message"},
		{exit.OnUnsubscribe, "leaving the series' topic"},
	} {
		if on.set {
			leaves = append(leaves, on.what)
		}
	}
	if len(leaves) > 0 {
		fmt.Fprintf(b, "  Exit: on %s\n", strings.Join(leaves, ", "))
	}
}

/*
writeGoal writes the goal, when the series has one.
*/
func writeGoal(b *strings.Builder, g *series.Goal) {
	if g != nil {
		what := "the event " + g.Event
		if g.Event == "" {
			what = "matching the rules " + truncate(compactJSON(g.Rules), 1000)
		}
		fmt.Fprintf(b, "  Goal: %s within %d days of entering", what, g.Days)
		if g.ControlPercent > 0 {
			fmt.Fprintf(b, " · %d%% of entrants are held out as a control group", g.ControlPercent)
		}
		b.WriteString("\n")
	}
}

/*
writeEntryCheck writes how the live series' rules judge the contact
now.
*/
func writeEntryCheck(b *strings.Builder, c series.EntryCheck) {
	b.WriteString("  How the live version judges this contact right now:\n")
	if c.Blocked {
		b.WriteString("    The contact is blocked, so they never enter.\n")
	}
	if c.NoCompany {
		b.WriteString("    The contact has no company, so this account series cannot enroll them.\n")
	}
	if c.Audience != nil {
		fmt.Fprintf(b, "    Entry audience %q (%s):\n", c.Audience.Name,
			models.PublicID(models.PrefixAudience, c.Audience.ID))
	}
	if c.Rules != nil {
		writeRuleCheck(b, "Entry rules", *c.Rules)
	}
	if c.Event != "" {
		done := "has not done it since the series went live"
		if c.EventDone {
			done = "did it since the series went live"
		}
		fmt.Fprintf(b, "    Entry event %s: the contact %s\n", c.Event, done)
	}
	if c.Exit != nil {
		writeRuleCheck(b, "Exit rules (a match keeps them out, or ends their journey)", *c.Exit)
	}
}

/*
writeRuleCheck writes one set of rules tested on the contact, condition
by condition.
*/
func writeRuleCheck(b *strings.Builder, what string, c series.RuleCheck) {
	if c.Problem != "" {
		fmt.Fprintf(b, "    %s: could not be tested: %s\n", what, c.Problem)
		return
	}
	fmt.Fprintf(b, "    %s: %s\n", what, matchWord(c.Result))
	for _, leaf := range c.Leaves {
		writeLeaf(b, leaf)
	}
}

/*
writeLeaf writes one condition with the value the contact has.
*/
func writeLeaf(b *strings.Builder, leaf audience.LeafResult) {
	fmt.Fprintf(b, "      - %s %s", leaf.Field, leaf.Op)
	if len(leaf.Value) > 0 {
		fmt.Fprintf(b, " %s", truncate(compactJSON(leaf.Value), 200))
	}
	actual, _ := json.Marshal(leaf.Actual)
	fmt.Fprintf(b, ": %s · the contact has %s", leaf.Result, truncate(string(actual), 200))
	if leaf.Trust != "" {
		fmt.Fprintf(b, " (%s)", leaf.Trust)
	}
	b.WriteString("\n")
}

/*
matchWord says what a rule's result means for the contact.
*/
func matchWord(result string) string {
	switch result {
	case "true":
		return "the contact matches"
	case "false":
		return "the contact does not match; each condition that is false below is why"
	default:
		return "it depends on something only a browser knows, such as the page"
	}
}
