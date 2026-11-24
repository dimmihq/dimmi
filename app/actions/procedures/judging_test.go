package procedures

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
recordingJudge answers every check it is given and keeps what it was
asked.
*/
type recordingJudge struct {
	asked []pipeline.Rubrics
	err   error
}

func (j *recordingJudge) JudgeRubrics(_ context.Context, in pipeline.Rubrics) ([]procedure.Judgment, error) {
	j.asked = append(j.asked, in)
	if j.err != nil {
		return nil, j.err
	}
	out := make([]procedure.Judgment, len(in.Rubrics))
	for i, rubric := range in.Rubrics {
		out[i] = procedure.Judgment{Passed: i%2 == 0, Message: "about " + rubric}
	}
	return out, nil
}

func TestJudgingARun(t *testing.T) {
	t.Parallel()

	rubric := func(text string) procedure.Assertion {
		return procedure.Assertion{Type: procedure.AssertLLMRubric, Rubric: text}
	}
	no := false
	checks := []procedure.Assertion{
		{Type: procedure.AssertHandedOff, HandedOff: &no},
		rubric("names the price"), rubric("asks one question"),
		rubric("names the price"),
	}

	t.Run("judges a run's written checks together, once, and gives each its own verdict", func(t *testing.T) {
		t.Parallel()
		judge := &recordingJudge{}
		p := &Procedures{Judge: judge}
		transcript := "CUSTOMER: hi\nRIO: Growth is $49 a month."

		results := procedure.Assert(checks, procedure.Observed{}, transcript,
			p.judge(t.Context(), authz.Membership{}, judging{
				checks: checks, transcript: transcript, wrote: []string{"claude-haiku-5-5"},
			}))

		require.Len(t, judge.asked, 1, "one call for the whole run")
		assert.Equal(t, []string{"names the price", "asks one question"}, judge.asked[0].Rubrics,
			"each written check once, the others left to code")
		assert.Equal(t, transcript, judge.asked[0].Transcript)
		assert.Equal(t, []string{"claude-haiku-5-5"}, judge.asked[0].Wrote,
			"the judge is told which model wrote the replies, so it is not that one")
		require.Len(t, results, 4)
		assert.True(t, results[1].Passed)
		assert.Equal(t, "about names the price", results[1].Message)
		assert.False(t, results[2].Passed)
		assert.Equal(t, "about asks one question", results[2].Message)
		assert.Equal(t, results[1].Message, results[3].Message, "a check written twice reads the same verdict")
	})

	t.Run("gives every written check no verdict when the judge could not answer", func(t *testing.T) {
		t.Parallel()
		judge := &recordingJudge{err: errors.New("down")}
		p := &Procedures{Judge: judge}

		results := procedure.Assert(checks, procedure.Observed{}, "",
			p.judge(t.Context(), authz.Membership{}, judging{checks: checks}))

		require.Len(t, judge.asked, 1, "it is not asked again for each check")
		for _, i := range []int{1, 2, 3} {
			assert.False(t, results[i].Passed)
			require.Error(t, results[i].Err)
		}
		assert.NotEmpty(t, unjudged(results), "the run reports that it has no verdict")
	})

	t.Run("has no judge when none is set up", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, (&Procedures{}).judge(t.Context(), authz.Membership{}, judging{checks: checks}))
	})
}
