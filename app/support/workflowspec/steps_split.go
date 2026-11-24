/*
|--------------------------------------------------------------------------
| A/B Split Steps
|--------------------------------------------------------------------------
|
| A split sends each conversation down one of two to four variants, by
| the shares the team gave them. Each variant is one of the step's
| paths, sharing its id. The pick is stable: a hash of the conversation
| and the step, so a conversation that meets the step again takes the
| same variant, and a dry run takes the one a live run would. Once the
| team picks a winner, everyone goes down it.
|
*/

package workflowspec

import (
	"hash/fnv"
	"strings"
	"unicode/utf8"
)

/*
KindSplit is the A/B split step.
*/
const KindSplit = "split"

/*
The bounds of a split.
*/
const (
	minVariants = 2
	maxVariants = 4
	totalShares = 100
)

/*
SplitSettings are the variants of a split and, once picked, the winner
everyone takes.
*/
type SplitSettings struct {
	Variants []SplitVariant `json:"variants"`
	Winner   string         `json:"winner,omitempty"`
}

/*
SplitVariant is one variant: the id of its path, a label and its share
of conversations, out of 100.
*/
type SplitVariant struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Weight int    `json:"weight"`
}

/*
splitKinds returns the split step kind.
*/
func splitKinds() []Kind {
	return []Kind{{Key: KindSplit, MinPaths: minVariants, MaxPaths: maxVariants, Ready: true}}
}

/*
Pick returns the variant the conversation key takes at the step, and
whether it is the winner rather than a variant picked by share. It is
empty when the split has no variant with a share.
*/
func (s SplitSettings) Pick(key, stepID string) (SplitVariant, bool) {
	if s.Winner != "" {
		for _, v := range s.Variants {
			if v.ID == s.Winner {
				return v, true
			}
		}
	}

	total := 0
	for _, v := range s.Variants {
		total += max(v.Weight, 0)
	}

	if total == 0 {
		return SplitVariant{}, false
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(key + ":" + stepID))
	pick := int(h.Sum32() % uint32(total))

	for _, v := range s.Variants {
		w := max(v.Weight, 0)
		if pick < w {
			return v, false
		}

		pick -= w
	}

	return SplitVariant{}, false
}

/*
splitChecks returns the check of the split step.
*/
func (c *checker) splitChecks() map[string]func(Step) {
	return map[string]func(Step){KindSplit: c.checkSplit}
}

/*
checkSplit checks a split's variants: two to four, each one of the
step's paths, labeled, with shares above 0 that add up to 100, and a
winner that is one of them.
*/
func (c *checker) checkSplit(s Step) {
	sp, ok := decode[SplitSettings](c, s)
	if !ok {
		return
	}

	bad := func(field, message string) {
		c.fail(Issue{Code: CodeInvalidSetting, StepID: s.ID, Field: field, Message: message})
	}

	if len(sp.Variants) < minVariants || len(sp.Variants) > maxVariants {
		c.fail(Issue{
			Code: CodeMissingSetting, StepID: s.ID, Field: "variants", Message: "A split has 2 to 4 variants.",
		})
	}

	paths := map[string]bool{}
	for _, p := range userPaths(s) {
		paths[p.ID] = true
	}

	seen, total := map[string]bool{}, 0
	for _, v := range sp.Variants {
		switch n := utf8.RuneCountInString(strings.TrimSpace(v.Label)); {
		case !paths[v.ID] || seen[v.ID]:
			bad("variants", "Give each variant its own path.")
		case n == 0 || n > maxLabel:
			bad("variants", "Label each variant, 40 characters at most.")
		case v.Weight < 1:
			bad("variants", "Give every variant a share above 0.")
		}

		seen[v.ID] = true
		total += v.Weight
	}

	if len(seen) != len(paths) {
		bad("variants", "Every path of a split is one of its variants.")
	}

	if len(sp.Variants) > 0 && total != totalShares {
		bad("variants", "The shares add up to 100.")
	}

	if sp.Winner != "" && !seen[sp.Winner] {
		bad("winner", "The winner must be one of the variants.")
	}
}
