/*
|--------------------------------------------------------------------------
| Checking Steps With Paths
|--------------------------------------------------------------------------
|
| Checks, splits and AI paths each name their own paths, and every other
| step has fixed ones. A path id is made once, like a step id, so the
| report can follow a path from one version to the next.
|
*/

package series

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

/*
day is the length of a day a wait counts in.
*/
const day = 24 * time.Hour

/*
condition checks a check has labeled paths that each test something.
*/
func (c *checker) condition(n Node) {
	s := settingsOf[ConditionSettings](n)
	if len(s.Paths) < 1 || len(s.Paths) > maxPaths-1 {
		c.add("condition_paths_invalid", at(n.ID, "paths"), "A check has 1 to %d paths besides everyone else.",
			maxPaths-1)
	}

	ids := map[string]bool{}
	for _, p := range s.Paths {
		c.pathID(n.ID, p.ID, ids)

		if len(p.Rules) == 0 && p.Engagement == nil {
			c.add("condition_path_empty", at(n.ID, "paths"), "The path %q tests nothing.", p.Label)
		}

		c.rules("condition", n.ID, "paths", p.Rules)
		c.engagement(n.ID, p.Engagement)
	}
}

/*
split checks a split has two to four variants with shares.
*/
func (c *checker) split(n Node) {
	s := settingsOf[SplitSettings](n)
	if len(s.Variants) < 2 || len(s.Variants) > 4 {
		c.add("split_variants_invalid", at(n.ID, "variants"), "A split has 2 to 4 variants.")
	}

	ids := map[string]bool{}
	for _, v := range s.Variants {
		c.pathID(n.ID, v.ID, ids)

		if v.Weight < 1 {
			c.add("split_weight_invalid", at(n.ID, "variants"), "Give every variant a share above 0.")
		}
	}

	if s.Winner != "" && !ids[s.Winner] {
		c.add("split_winner_invalid", at(n.ID, "winner"), "The winner must be one of the variants.")
	}
}

/*
aiPath checks the AI has a prompt and paths to pick from.
*/
func (c *checker) aiPath(n Node) {
	s := settingsOf[AIPathSettings](n)
	if strings.TrimSpace(s.Prompt) == "" {
		c.add("ai_prompt_missing", at(n.ID, "prompt"), "Tell the AI how to pick a path.")
	}

	if len(s.Paths) < 2 || len(s.Paths) > maxPaths-1 {
		c.add("ai_paths_invalid", at(n.ID, "paths"), "The AI picks from 2 to %d paths.", maxPaths-1)
	}

	ids := map[string]bool{}
	for _, p := range s.Paths {
		c.pathID(n.ID, p.ID, ids)
	}
}

/*
pathID checks a path a step names has a good, unique id.
*/
func (c *checker) pathID(node, id string, seen map[string]bool) {
	switch {
	case !nodeID.MatchString(id) || fixedPath(id):
		c.add("path_id_invalid", at(node, "paths"), "Path ids use lowercase letters, digits, - and _.")
	case seen[id]:
		c.add("path_id_taken", at(node, "paths"), "Two paths share the id %q.", id)
	}

	seen[id] = true
}

/*
action checks a step that changes the person says what to change.
*/
func (c *checker) action(n Node) {
	s := settingsOf[ActionSettings](n)

	switch n.Kind {
	case KindTag, KindUntag:
		if len(s.TagIDs) == 0 {
			c.add("tags_missing", at(n.ID, "tag_ids"), "Choose the tags.")
		}
	case KindSetAttribute:
		if !attributeKey.MatchString(s.Key) || len(s.Value) == 0 {
			c.add("attribute_invalid", at(n.ID, "key"), "Choose the attribute and the value to write.")
		}
	case KindTopic:
		if s.TopicID == "" {
			c.add("topic_missing", at(n.ID, "topic_id"), "Choose the topic.")
		}

		if c.subject == SubjectCompany {
			c.add("topic_company", at(n.ID, "topic_id"), "Topics belong to people, not to companies.")
		}
	case KindWebhook:
		w := settingsOf[WebhookSettings](n)
		if u, err := url.Parse(w.URL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			c.add("webhook_url_invalid", at(n.ID, "url"), "Give the full address the webhook posts to.")
		}
	}
}

/*
pathsOf lists the paths a step can leave by.
*/
func pathsOf(n Node) []string {
	switch {
	case n.Kind == KindExit:
		return nil
	case IsMessage(n.Kind):
		return []string{PathNext, PathFailed}
	case n.Kind == KindWait:
		return []string{PathNext, PathNoDate}
	case n.Kind == KindWaitFor:
		return []string{PathMatched, PathTimedOut}
	case n.Kind == KindCondition:
		return append(pathIDs(settingsOf[ConditionSettings](n).Paths, func(p ConditionPath) string { return p.ID }),
			PathElse)
	case n.Kind == KindSplit:
		return pathIDs(settingsOf[SplitSettings](n).Variants, func(v Variant) string { return v.ID })
	case n.Kind == KindAIPath:
		return append(pathIDs(settingsOf[AIPathSettings](n).Paths, func(p AIPath) string { return p.ID }), PathFallback)
	}

	return []string{PathNext}
}

/*
pathIDs lists the ids of a step's own paths.
*/
func pathIDs[T any](list []T, id func(T) string) []string {
	out := make([]string, 0, len(list)+1)
	for _, item := range list {
		out = append(out, id(item))
	}

	return out
}

/*
fixedPath reports whether an id is taken by a path every step of a kind
has.
*/
func fixedPath(id string) bool {
	switch id {
	case PathNext, PathFailed, PathMatched, PathTimedOut, PathElse, PathFallback, PathNoDate:
		return true
	}

	return false
}

/*
durationOf turns an amount of units into a duration.
*/
func durationOf(amount int, unit string) (time.Duration, bool) {
	units := map[string]time.Duration{
		"minutes": time.Minute, "hours": time.Hour, "days": day, "weeks": 7 * day,
	}

	u, ok := units[unit]
	if !ok {
		return 0, false
	}

	return time.Duration(amount) * u, true
}

/*
clockTime reads a time of day written HH:MM.
*/
func clockTime(at string) (int, int, bool) {
	hh, mm, found := strings.Cut(at, ":")
	if !found {
		return 0, 0, false
	}

	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)

	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}

	return h, m, true
}
