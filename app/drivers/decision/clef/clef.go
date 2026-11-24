/*
|--------------------------------------------------------------------------
| Clef Decider
|--------------------------------------------------------------------------
|
| Cloudflare's Clef answers choice, score and yes-or-no questions about
| a piece of state with a probability each, up to sixty-four questions
| a request. It runs on Workers AI, which keeps no request after
| answering it and trains on none. The driver translates the contract
| to the Workers AI REST API over the shared HTTP client, asks a longer
| list of questions as several requests at once, and maps its failures
| to errors the pipeline can record.
|
*/

package clef

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Errors the driver returns: ErrNotConfigured without an account id or
an API token, ErrRejected when the API refuses the request,
ErrUnavailable when it fails, is out of capacity or answers something
unreadable.
*/
var (
	ErrNotConfigured = errors.New("clef: no account id or api token")
	ErrRejected      = errors.New("clef: request rejected")
	ErrUnavailable   = errors.New("clef: unavailable")
)

/*
MaxQuestions is how many questions the API takes in one request, and
maxRequests how many requests one call sends at once.
*/
const (
	MaxQuestions = 64
	maxRequests  = 4
)

/*
The instructions sent for a question that came without any: the API
refuses a question with none.
*/
const (
	askChoice = "Choose the option that best describes the state."
	askScore  = "Rate the state on the scale."
	askNoul   = "Is this true of the state?"
)

/*
Config is one Cloudflare account: its id, an API token allowed to run
Workers AI, the API's base URL, the model asked for (clef or
clef-flash), and the timeout of one call.
*/
type Config struct {
	AccountID string
	APIToken  string
	BaseURL   string
	Model     string
	Timeout   time.Duration
}

/*
Driver is the Clef client.
*/
type Driver struct {
	http  *httpclient.Client
	clock clock.Clock
	cfg   Config
	url   string
}

/*
New returns the driver for an account.
*/
func New(http *httpclient.Client, clk clock.Clock, cfg Config) (*Driver, error) {
	cfg.AccountID, cfg.APIToken = strings.TrimSpace(cfg.AccountID), strings.TrimSpace(cfg.APIToken)
	if cfg.AccountID == "" || cfg.APIToken == "" {
		return nil, ErrNotConfigured
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.cloudflare.com/client/v4"
	}
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		cfg.Model = "clef"
	}
	address := strings.TrimSuffix(cfg.BaseURL, "/") + "/accounts/" + url.PathEscape(cfg.AccountID) +
		"/ai/run/@cf/cloudflare/" + url.PathEscape(cfg.Model)
	return &Driver{http: http, clock: clk, cfg: cfg, url: address}, nil
}

/*
wireRequest is the API's request body.
*/
type wireRequest struct {
	Model     string                  `json:"model"`
	State     string                  `json:"state"`
	Questions map[string]wireQuestion `json:"questions"`
}

/*
wireQuestion is one question as the API takes it: criteria is an
object for choice and noul, an array for score.
*/
type wireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

/*
wireEnvelope is what the REST API wraps an answer in. A self-hosted
Clef server answers without it, so both shapes are read.
*/
type wireEnvelope struct {
	Result  *wireResponse `json:"result"`
	Success *bool         `json:"success"`
	Errors  []wireError   `json:"errors"`
	wireResponse
}

/*
wireError is one error of a refused or failed call.
*/
type wireError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

/*
wireResponse is the model's answer.
*/
type wireResponse struct {
	Model   string                `json:"model"`
	Answers map[string]wireAnswer `json:"answers"`
	Usage   struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

/*
wireAnswer is one answer as the API sends it.
*/
type wireAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	Noul          float64            `json:"noul"`
}

/*
Decide asks the questions, in one request when the API takes them all
and otherwise in as many as it needs, a few at once, each with the same
state. Every question asked must come back answered with a probability
in range, or the call counts as failed: a missing answer read as zero
would be a confident no. When one request fails the whole call does,
so a caller never acts on half the answers.
*/
func (d *Driver) Decide(ctx context.Context, req contracts.DecisionRequest) (*contracts.DecisionResponse, error) {
	started := d.clock.Now()
	names := slices.Sorted(maps.Keys(req.Questions))
	if len(names) <= MaxQuestions {
		out, err := d.ask(ctx, req)
		if err != nil {
			return nil, err
		}
		return fromWire(out, d.clock.Now().Sub(started)), nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		failed error
		all    = wireResponse{Answers: make(map[string]wireAnswer, len(names))}
		slots  = make(chan struct{}, maxRequests)
	)
	for part := range slices.Chunk(names, MaxQuestions) {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			some := contracts.DecisionRequest{
				State: req.State, Questions: make(map[string]contracts.DecisionQuestion, len(part)),
			}
			for _, name := range part {
				some.Questions[name] = req.Questions[name]
			}
			out, err := d.ask(ctx, some)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if failed == nil {
					failed = err
					cancel()
				}
				return
			}
			maps.Copy(all.Answers, out.Answers)
			all.Model = out.Model
			all.Usage.InputTokens += out.Usage.InputTokens
			all.Usage.OutputTokens += out.Usage.OutputTokens
		})
	}
	wg.Wait()
	if failed != nil {
		return nil, failed
	}
	return fromWire(all, d.clock.Now().Sub(started)), nil
}

/*
ask sends one request of at most MaxQuestions questions and returns
its checked answer.
*/
func (d *Driver) ask(ctx context.Context, req contracts.DecisionRequest) (wireResponse, error) {
	var none wireResponse
	body := wireRequest{Model: d.cfg.Model, State: req.State, Questions: map[string]wireQuestion{}}
	for name, q := range req.Questions {
		body.Questions[name] = toWire(q)
	}

	call := d.http.New().WithToken(d.cfg.APIToken).Accept("application/json")
	if d.cfg.Timeout > 0 {
		call = call.Timeout(d.cfg.Timeout)
	}
	resp, err := call.Post(ctx, d.url, body)
	if err != nil {
		return none, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	var env wireEnvelope
	readable := json.Unmarshal(resp.Body, &env) == nil
	detail := failure(env, resp.Body)
	switch {
	case resp.Status == 400 || resp.Status == 401 || resp.Status == 403 || resp.Status == 404 || resp.Status == 422:
		return none, fmt.Errorf("%w: status %d: %s", ErrRejected, resp.Status, detail)
	case resp.Status < 200 || resp.Status > 299:
		return none, fmt.Errorf("%w: status %d: %s", ErrUnavailable, resp.Status, detail)
	case !readable:
		return none, fmt.Errorf("%w: unreadable answer", ErrUnavailable)
	case env.Success != nil && !*env.Success:
		return none, fmt.Errorf("%w: %s", ErrUnavailable, detail)
	}

	out := env.wireResponse
	if env.Result != nil {
		out = *env.Result
	}
	if err := complete(req, out); err != nil {
		return none, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if out.Model == "" {
		out.Model = d.cfg.Model
	}
	return out, nil
}

/*
toWire maps one question to the API's shape, with stock instructions
when it brought none.
*/
func toWire(q contracts.DecisionQuestion) wireQuestion {
	w := wireQuestion{Type: string(q.Type), Instructions: strings.TrimSpace(q.Instructions)}
	switch q.Type {
	case contracts.DecisionChoice:
		w.Criteria = q.Choices
		if w.Instructions == "" {
			w.Instructions = askChoice
		}
	case contracts.DecisionScore:
		w.Criteria = q.Levels
		if w.Instructions == "" {
			w.Instructions = askScore
		}
	case contracts.DecisionNoul:
		if q.True != "" || q.False != "" {
			w.Criteria = map[string]string{"true": q.True, "false": q.False}
		}
		if w.Instructions == "" {
			w.Instructions = askNoul
		}
	}
	return w
}

/*
complete reports the first question the answer leaves out or answers
with another type or a probability outside zero to one.
*/
func complete(req contracts.DecisionRequest, out wireResponse) error {
	if len(out.Answers) == 0 {
		return errors.New("no answers")
	}
	for name, q := range req.Questions {
		a, ok := out.Answers[name]
		switch {
		case !ok:
			return fmt.Errorf("no answer to %q", name)
		case a.Type != "" && a.Type != string(q.Type):
			return fmt.Errorf("answer to %q is a %s, not a %s", name, a.Type, q.Type)
		case a.Noul < 0 || a.Noul > 1 || a.Confidence < 0 || a.Confidence > 1:
			return fmt.Errorf("answer to %q has a probability out of range", name)
		}
	}
	return nil
}

/*
fromWire maps the API's answer to the contract.
*/
func fromWire(w wireResponse, latency time.Duration) *contracts.DecisionResponse {
	out := &contracts.DecisionResponse{
		Model: w.Model, Answers: map[string]contracts.DecisionAnswer{}, Latency: latency,
		Usage: contracts.DecisionUsage{InputTokens: w.Usage.InputTokens, OutputTokens: w.Usage.OutputTokens},
	}
	for name, a := range w.Answers {
		out.Answers[name] = contracts.DecisionAnswer{
			Type: contracts.DecisionType(a.Type), Choice: a.Choice, Score: a.Score,
			Confidence: a.Confidence, Probabilities: a.Probabilities, Noul: a.Noul,
		}
	}
	return out
}

/*
failure is the short reason of a call that went wrong: the API's own
errors when it sent any, else the start of the body.
*/
func failure(env wireEnvelope, body []byte) string {
	if len(env.Errors) == 0 {
		return truncate(string(body))
	}
	parts := make([]string, 0, len(env.Errors))
	for _, e := range env.Errors {
		parts = append(parts, fmt.Sprintf("%d %s", e.Code, e.Message))
	}
	return truncate(strings.Join(parts, "; "))
}

/*
truncate keeps an error body short.
*/
func truncate(s string) string {
	if len(s) > 300 {
		return s[:300]
	}
	return s
}
