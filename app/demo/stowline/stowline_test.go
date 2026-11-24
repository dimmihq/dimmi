package stowline_test

import (
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/ai/costsim"
	"github.com/dimmihq/dimmi/app/demo/stowline"
)

var (
	articleSlug = regexp.MustCompile(`^[a-z0-9-]{1,120}$`)
	placeholder = regexp.MustCompile(`\{(first|me|order|tracking|carrier|platform|amount|plan|code|shipment|invoice)\}`)
)

/*
planAt is the moment plans are made at in these tests.
*/
func planAt() time.Time {
	return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
}

func small(t *testing.T) stowline.Size {
	t.Helper()
	size, err := stowline.SizeNamed("small")
	require.NoError(t, err)
	return size
}

func TestHelpCenter(t *testing.T) {
	t.Parallel()

	t.Run("has about 400 articles of real length in every section", func(t *testing.T) {
		t.Parallel()
		articles := stowline.Articles()
		require.GreaterOrEqual(t, len(articles), 400)
		total := 0
		for _, a := range articles {
			total += stowline.WordCount(a.Body)
		}
		assert.Greater(t, total, 300_000, "at least 300,000 words of knowledge")

		sections := map[string]int{}
		long := 0
		for _, a := range articles {
			sections[a.Collection]++
			words := stowline.WordCount(a.Body)
			if words >= 4_000 {
				long++
			}
		}
		for _, c := range stowline.Collections() {
			assert.Positive(t, sections[c.Slug], c.Slug)
		}
		assert.Len(t, sections, len(stowline.Collections()), "every article sits in a known section")
		assert.GreaterOrEqual(t, long, 20, "long documents of 4,000 words and more")
	})

	t.Run("keeps every article within the help center's limits", func(t *testing.T) {
		t.Parallel()
		seen := map[string]bool{}
		for _, a := range stowline.Articles() {
			assert.Regexp(t, articleSlug, a.Slug)
			assert.False(t, seen[a.Slug], "duplicate slug %s", a.Slug)
			seen[a.Slug] = true
			assert.LessOrEqual(t, len([]rune(a.Title)), 150)
			assert.LessOrEqual(t, len([]rune(a.Description)), 300)
			assert.Contains(t, []string{"public", "agent_only"}, a.Visibility)
			assert.NotRegexp(t, placeholder, a.Body, a.Title)
		}
	})

	t.Run("writes the same articles every time", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, stowline.Articles(), stowline.Articles())
	})
}

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("is the same for the same seed and moment", func(t *testing.T) {
		t.Parallel()
		a := stowline.NewPlan(small(t), 7, planAt())
		b := stowline.NewPlan(small(t), 7, planAt())
		assert.Equal(t, a, b)
	})

	t.Run("differs for another seed", func(t *testing.T) {
		t.Parallel()
		a := stowline.NewPlan(small(t), 7, planAt())
		b := stowline.NewPlan(small(t), 8, planAt())
		assert.NotEqual(t, a.Conversations[0].Messages, b.Conversations[0].Messages)
	})

	t.Run("plans the size it was asked for with unique ids and emails", func(t *testing.T) {
		t.Parallel()
		size := small(t)
		p := stowline.NewPlan(size, 7, planAt())
		require.Len(t, p.Companies, size.Companies)
		require.Len(t, p.Contacts, size.Contacts)
		require.Len(t, p.Conversations, size.Conversations+stowline.PlantedCases())
		require.Len(t, p.Team, size.Teammates)

		checkUnique(t, p)
	})

	t.Run("spreads conversations over the year, every channel, mostly short with a long tail", func(t *testing.T) {
		t.Parallel()
		p := stowline.NewPlan(small(t), 7, planAt())
		seen := map[string]int{}
		short, long := 0, 0
		for _, c := range p.Conversations {
			seen["channel:"+c.Channel]++
			seen["status:"+c.Status]++
			seen["ai:"+c.AI]++
			if c.Rating > 0 {
				seen["rated"]++
			}
			switch n := len(c.Messages); {
			case n <= 4:
				short++
			case n >= 20:
				long++
			}
			checkConversation(t, c)
		}
		for _, k := range []string{
			"channel:widget", "channel:email", "channel:api", "channel:slack", "status:open", "status:closed",
			"status:snoozed", "ai:resolved", "ai:handed_off", "rated",
		} {
			assert.Positive(t, seen[k], k)
		}
		assert.Greater(t, short, len(p.Conversations)*2/5, "most are short")
		assert.Positive(t, long, "a long tail of 20+ messages")
	})

	t.Run("knows its sizes and refuses others", func(t *testing.T) {
		t.Parallel()
		for _, s := range stowline.Sizes() {
			got, err := stowline.SizeNamed(s.Name)
			require.NoError(t, err)
			assert.Equal(t, s, got)
		}
		large, err := stowline.SizeNamed("large")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, large.Conversations, 10_000)
		_, err = stowline.SizeNamed("huge")
		assert.Error(t, err)
	})
}

func TestScenarios(t *testing.T) {
	t.Parallel()

	t.Run("covers every kind with at least 40 scenarios", func(t *testing.T) {
		t.Parallel()
		scenarios := stowline.Scenarios()
		require.GreaterOrEqual(t, len(scenarios), 40)
		kinds, keys := map[string]int{}, map[string]bool{}
		troubleshootingTurns := 0
		for _, s := range scenarios {
			kinds[s.Kind]++
			assert.False(t, keys[s.Key], "duplicate scenario %s", s.Key)
			keys[s.Key] = true
			assert.NotEmpty(t, s.Messages, s.Key)
			assert.NotEmpty(t, s.Persona, s.Key)
			assert.NotEmpty(t, s.Goal, s.Key)
			if s.Kind == costsim.KindTroubleshooting {
				troubleshootingTurns = max(troubleshootingTurns, len(s.Messages))
			}
		}
		for _, k := range costsim.Kinds() {
			assert.Positive(t, kinds[k], k)
		}
		assert.GreaterOrEqual(t, troubleshootingTurns, 8)
	})

	t.Run("answers the actions' API routes and refuses others", func(t *testing.T) {
		t.Parallel()
		for _, a := range stowline.AgentActions([16]byte{}) {
			assert.True(t, slices.Contains([]string{"read", "write"}, *a.SideEffect))
		}
		client, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)
		stowline.StubAPI(client)

		res, err := client.New().Get(t.Context(), stowline.APIBaseURL+"/trackers/1ZX4F1E40312345678")
		require.NoError(t, err)
		assert.Equal(t, 200, res.Status)
		assert.Contains(t, string(res.Body), "in_transit")

		_, err = client.New().Get(t.Context(), stowline.APIBaseURL+"/nowhere")
		assert.Error(t, err)
	})
}

/*
checkConversation checks one planned conversation reads like a real
one: within the year, in order, nothing in the future, opened by the
customer, rated only when closed, and every placeholder filled.
*/
func checkConversation(t *testing.T, c stowline.Conversation) {
	t.Helper()
	assert.False(t, c.CreatedAt.Before(planAt().Add(-stowline.History-24*time.Hour)), "within the year")
	assert.Equal(t, "contact", c.Messages[0].Author, "the customer opens")
	assert.True(t, c.Rating == 0 || c.Status == "closed", "only closed conversations are rated")
	assert.True(t, c.Status != "closed" || !c.ClosedAt.IsZero(), "a closed conversation has its time")
	for i, m := range c.Messages {
		assert.False(t, m.At.After(planAt()), "no message in the future")
		assert.True(t, i == 0 || !m.At.Before(c.Messages[i-1].At), "messages in order")
		assert.NotRegexp(t, placeholder, m.Body)
	}
}

/*
checkUnique checks no two people or conversations share an id and no
two people share an email.
*/
func checkUnique(t *testing.T, p stowline.Plan) {
	t.Helper()
	seen := map[string]bool{}
	for _, c := range p.Contacts {
		assert.False(t, seen[c.ID.String()], "duplicate id")
		seen[c.ID.String()] = true
		if c.Email != "" {
			assert.False(t, seen[c.Email], "duplicate email %s", c.Email)
			seen[c.Email] = true
		}
	}
	for _, c := range p.Conversations {
		assert.False(t, seen[c.ID.String()], "duplicate id")
		seen[c.ID.String()] = true
	}
}

func TestPlantedCases(t *testing.T) {
	t.Parallel()

	p := stowline.NewPlan(small(t), 7, planAt())
	planted := p.Conversations[len(p.Conversations)-stowline.PlantedCases():]

	bySubject := map[string]int{}
	waiting := 0

	for _, c := range planted {
		bySubject[c.Subject]++

		last := c.Messages[len(c.Messages)-1]
		if c.Status == "open" && last.Author == "contact" {
			waiting++

			assert.Greater(t, p.Now.Sub(last.At), 20*time.Hour-time.Minute, "%s has waited", c.Subject)
		}

		assert.False(t, c.CreatedAt.After(p.Now), "%s starts before now", c.Subject)
	}

	assert.Equal(t, 6, waiting, "four who need a person today and two small ones")
	assert.Equal(t, 4, bySubject["Pausing our plan"], "the knowledge gap")
	assert.Equal(t, 2, bySubject["UPS to a PO box"], "the question answered two ways")
	assert.Equal(t, 5, bySubject["Export to Google Sheets"], "the feature request")

	titles := map[string]bool{}
	for _, a := range stowline.Articles() {
		titles[a.Title] = true
	}

	assert.True(t, titles["Lost parcel checklist"], "the article that disagrees on the claims window")
}
