package ai_test

import (
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/decision/clef"
	"github.com/dimmihq/dimmi/tests"
)

/*
liveClef is the real Clef driver on the account the environment names,
with the timeout production gives one call. The live tests run only
with DIMMI_LIVE_CLEF=1, CLOUDFLARE_ACCOUNT_ID and a
CLOUDFLARE_WORKERS_AI_TOKEN allowed to run Workers AI; every run spends
real money, a cent or less, and CI never sets them. DIMMI_LIVE_CLEF_MODEL
picks clef-flash over clef.
*/
func liveClef(t *testing.T) *clef.Driver {
	t.Helper()
	if !testkit.Switch("DIMMI_LIVE_CLEF") {
		t.Skip("live clef tests run only with DIMMI_LIVE_CLEF=1, CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_WORKERS_AI_TOKEN")
	}
	tc := tests.New(t)
	d, err := clef.New(tc.App.HTTP, tc.App.Clock, clef.Config{
		AccountID: testkit.Env(t, "CLOUDFLARE_ACCOUNT_ID"), APIToken: testkit.Env(t, "CLOUDFLARE_WORKERS_AI_TOKEN"),
		Model: os.Getenv("DIMMI_LIVE_CLEF_MODEL"), Timeout: 3 * time.Second,
	})
	require.NoError(t, err)
	return d
}

/*
decideTwice asks the way the pipeline does: once, and once more when
the first call failed. It reports the answer and whether the first try
was the one that answered.
*/
func decideTwice(t *testing.T, d *clef.Driver, req contracts.DecisionRequest) (*contracts.DecisionResponse, bool) {
	t.Helper()
	res, err := d.Decide(t.Context(), req)
	if err == nil {
		return res, true
	}
	t.Logf("first try failed: %v", err)
	res, err = d.Decide(t.Context(), req)
	require.NoError(t, err)
	return res, false
}

/*
TestLiveClefAnswersEveryType asks one question of each type in one
request and reads a probability back for each.
*/
func TestLiveClefAnswersEveryType(t *testing.T) {
	d := liveClef(t)
	res, _ := decideTwice(t, d, contracts.DecisionRequest{
		State: "<draft_reply>Your refund of $20 was sent today. Could you confirm the last four numbers " +
			"of your card?</draft_reply>",
		Questions: map[string]contracts.DecisionQuestion{
			"asks": {Type: contracts.DecisionNoul, Instructions: "Does the draft reply ask the customer a question?"},
			"tone": {
				Type: contracts.DecisionChoice, Instructions: "What is the tone of the reply?",
				Choices: map[string]string{"polite": "Courteous", "rude": "Hostile or dismissive"},
			},
			"clarity": {
				Type: contracts.DecisionScore, Instructions: "How clear is the reply?",
				Levels: []string{"Confusing", "Somewhat clear", "Very clear"},
			},
			"bare": {Type: contracts.DecisionChoice, Choices: map[string]string{"refund": "", "shipping": ""}},
		},
	})
	t.Logf("model %s, %d input tokens: %+v", res.Model, res.Usage.InputTokens, res.Answers)

	assert.Greater(t, res.Answers["asks"].Noul, 0.85)
	assert.Equal(t, "polite", res.Answers["tone"].Choice)
	assert.InDelta(t, 1, res.Answers["tone"].Probabilities["polite"]+res.Answers["tone"].Probabilities["rude"], 0.02)
	assert.GreaterOrEqual(t, res.Answers["clarity"].Score, 1.0)
	assert.Equal(t, "refund", res.Answers["bare"].Choice)
	assert.Positive(t, res.Usage.InputTokens)
}

/*
liveCase is one judgment with the answer a person would give.
*/
type liveCase struct {
	language string
	state    string
	want     bool
}

/*
TestLiveClefJudgesInManyLanguages asks the two questions the pipeline
leans on most, whether the customer wants a person and whether a reply
asks something, of messages in eleven languages, a yes and a no each.
*/
func TestLiveClefJudgesInManyLanguages(t *testing.T) {
	d := liveClef(t)

	wantsPerson := contracts.DecisionQuestion{
		Type:         contracts.DecisionNoul,
		Instructions: "Is the customer asking to talk to a human member of the team instead of the AI agent?",
		True:         "The customer asks for a person, an agent, a representative or a manager.",
		False:        "The customer asks something else, or only describes a problem.",
	}
	asks := contracts.DecisionQuestion{
		Type:         contracts.DecisionNoul,
		Instructions: "Does the reply ask the customer for something it needs an answer to?",
		True:         "The reply asks a question or requests a detail from the customer.",
		False:        "The reply only informs, confirms or closes.",
	}

	person := []liveCase{
		{"English", "I have explained this three times. Let me talk to a real person please.", true},
		{"English", "My order arrived with a cracked screen. What are my options?", false},
		{"French", "Je veux parler à un conseiller humain, pas à un robot.", true},
		{"French", "Ma facture indique un montant incorrect ce mois-ci.", false},
		{"Spanish", "Por favor, páseme con una persona de su equipo.", true},
		{"Spanish", "¿Cuánto tarda el envío a Sevilla?", false},
		{"Portuguese", "Quero falar com um atendente de verdade, por favor.", true},
		{"Portuguese", "Não consigo redefinir a minha senha.", false},
		{"German", "Bitte verbinden Sie mich mit einem Mitarbeiter.", true},
		{"German", "Wann wird meine Bestellung geliefert?", false},
		{"Arabic", "أريد التحدث مع موظف حقيقي من فضلكم.", true},
		{"Arabic", "لم يصلني رمز التحقق بعد.", false},
		{"Japanese", "担当者の方と直接お話しさせてください。", true},
		{"Japanese", "領収書をもう一度送ってもらえますか。", false},
		{"Hindi", "मुझे किसी असली इंसान से बात करनी है।", true},
		{"Hindi", "मेरा ऑर्डर अभी तक नहीं पहुँचा है।", false},
		{"Yoruba", "Mo fẹ́ bá ènìyàn gidi sọ̀rọ̀, kì í ṣe ẹ̀rọ.", true},
		{"Yoruba", "Owó mi kò tíì padà sí àkáǹtì mi.", false},
		{"Nigerian Pidgin", "Abeg, make una give me real person make I follow am talk.", true},
		{"Nigerian Pidgin", "Di money wey I send never enter since yesterday.", false},
		{"Swahili", "Naomba kuongea na mhudumu halisi, tafadhali.", true},
		{"Swahili", "Sijapokea bidhaa niliyoagiza wiki iliyopita.", false},
	}
	replies := []liveCase{
		{"English", "Could you share the order number so I can look into it?", true},
		{"English", "Your refund was issued today and will arrive within five days.", false},
		{"French", "Pouvez-vous me donner l'adresse e-mail liée à votre compte ?", true},
		{"French", "Votre abonnement a bien été annulé.", false},
		{"Spanish", "¿Me puede indicar el número de pedido?", true},
		{"Spanish", "Hemos actualizado su dirección de envío.", false},
		{"Arabic", "هل يمكنك إرسال رقم الطلب؟", true},
		{"Arabic", "تم إلغاء اشتراكك بنجاح.", false},
		{"Japanese", "ご注文番号を教えていただけますか。", true},
		{"Japanese", "返金の手続きが完了しました。", false},
		{"Nigerian Pidgin", "Abeg which phone number you take register?", true},
		{"Nigerian Pidgin", "We don reverse di charge, e go show for your account soon.", false},
	}

	wrong := 0
	run := func(kind, tag string, q contracts.DecisionQuestion, cases []liveCase) {
		for _, c := range cases {
			res, _ := decideTwice(t, d, contracts.DecisionRequest{
				State:     "<" + tag + ">" + c.state + "</" + tag + ">",
				Questions: map[string]contracts.DecisionQuestion{"q": q},
			})
			p := res.Answers["q"].Noul
			right := (p >= 0.5) == c.want
			if !right {
				wrong++
			}
			t.Logf("%-14s %-16s want %-5t p=%.3f sure=%-5t right=%t  %s",
				kind, c.language, c.want, p, p >= 0.85 || p <= 0.15, right, c.state)
			assert.Equal(t, c.want, p >= 0.5, "%s in %s: %s", kind, c.language, c.state)
		}
	}
	run("wants a person", "customer_message", wantsPerson, person)
	run("reply asks", "draft_reply", asks, replies)
	t.Logf("%d of %d judgments wrong", wrong, len(person)+len(replies))
}

/*
TestLiveClefHoldsUnderLoad asks sixty requests four at a time, with the
timeout production gives one call, and expects the pipeline's two tries
to always get an answer, nearly always on the first, and the same
answer every time the same question is asked.
*/
func TestLiveClefHoldsUnderLoad(t *testing.T) {
	d := liveClef(t)
	const calls, workers = 60, 4

	req := contracts.DecisionRequest{
		State: "<customer_message>I was charged twice for my subscription this month and I want one " +
			"payment back.</customer_message>",
		Questions: map[string]contracts.DecisionQuestion{
			"billing": {Type: contracts.DecisionNoul, Instructions: "Is the message about a billing problem?"},
			"person":  {Type: contracts.DecisionNoul, Instructions: "Does the customer ask to talk to a human?"},
			"angry":   {Type: contracts.DecisionNoul, Instructions: "Is the customer insulting the team?"},
		},
	}

	var (
		mu        sync.Mutex
		latencies []time.Duration
		billing   []float64
		retried   int
	)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range jobs {
				started := time.Now()
				res, first := decideTwice(t, d, req)
				took := time.Since(started)
				mu.Lock()
				latencies = append(latencies, took)
				billing = append(billing, res.Answers["billing"].Noul)
				if !first {
					retried++
				}
				mu.Unlock()
				assert.Greater(t, res.Answers["billing"].Noul, 0.5)
				assert.Less(t, res.Answers["person"].Noul, 0.5)
				assert.Less(t, res.Answers["angry"].Noul, 0.5)
			}
		})
	}
	for i := range calls {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	slices.Sort(latencies)
	slices.Sort(billing)
	p := func(q float64) time.Duration { return latencies[int(float64(len(latencies)-1)*q)] }
	t.Logf("%d calls: p50 %s, p95 %s, max %s; %d needed the second try; billing p from %.3f to %.3f",
		len(latencies), p(0.5), p(0.95), latencies[len(latencies)-1], retried, billing[0], billing[len(billing)-1])

	require.Len(t, latencies, calls)
	assert.LessOrEqual(t, retried, calls/20, "more than one call in twenty needed the second try")
	assert.Less(t, p(0.95), 2*time.Second)
	assert.InDelta(t, billing[0], billing[len(billing)-1], 0.05, "the same question drew different answers")
}

/*
TestLiveClefTakesAFullRequest asks as many questions as one request
takes about a long state and expects every one answered.
*/
func TestLiveClefTakesAFullRequest(t *testing.T) {
	d := liveClef(t)
	state := "<conversation>"
	for i := range 40 {
		state += fmt.Sprintf("\n<customer_message>Message %d: my invoice for March shows the wrong VAT number "+
			"and I need a corrected copy for my accountant.</customer_message>", i)
	}
	req := contracts.DecisionRequest{
		State: state + "\n</conversation>", Questions: map[string]contracts.DecisionQuestion{},
	}
	for i := range clef.MaxQuestions {
		req.Questions[fmt.Sprintf("q%02d", i)] = contracts.DecisionQuestion{
			Type: contracts.DecisionNoul, Instructions: "Is the conversation about an invoice?",
		}
	}
	started := time.Now()
	res, _ := decideTwice(t, d, req)
	t.Logf("%d questions, %d input tokens, %s", len(res.Answers), res.Usage.InputTokens, time.Since(started))
	require.Len(t, res.Answers, clef.MaxQuestions)
	for name, a := range res.Answers {
		assert.Greater(t, a.Noul, 0.5, name)
	}
}

/*
TestLiveClefSplitsALongList asks more questions than one request takes,
the way a workspace with many procedures does, and expects every one
answered in about the time of one request.
*/
func TestLiveClefSplitsALongList(t *testing.T) {
	d := liveClef(t)
	const asked = 2*clef.MaxQuestions + 22
	req := contracts.DecisionRequest{
		State:     "<customer_message>My invoice for March shows the wrong VAT number.</customer_message>",
		Questions: map[string]contracts.DecisionQuestion{},
	}
	for i := range asked {
		ask := "Is the message about an invoice?"
		if i%2 == 1 {
			ask = "Is the message about a late delivery?"
		}
		req.Questions[fmt.Sprintf("q%03d", i)] = contracts.DecisionQuestion{
			Type: contracts.DecisionNoul, Instructions: ask,
		}
	}
	started := time.Now()
	res, _ := decideTwice(t, d, req)
	t.Logf("%d questions, %d input tokens, %s", len(res.Answers), res.Usage.InputTokens, time.Since(started))
	require.Len(t, res.Answers, asked)
	for i := range asked {
		name := fmt.Sprintf("q%03d", i)
		assert.Equal(t, i%2 == 0, res.Answers[name].Noul >= 0.5, name)
	}
}
