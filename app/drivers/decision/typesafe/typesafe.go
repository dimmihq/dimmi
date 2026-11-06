/*
|--------------------------------------------------------------------------
| TypeSafe Decider
|--------------------------------------------------------------------------
|
| TypeSafe's System One answers choice, score and yes-or-no questions
| about a piece of state with calibrated probabilities, in one request
| for any number of questions. The driver translates the contract to
| its JSON API over the shared HTTP client and maps its failures to
| errors the pipeline can record.
|
*/

package typesafe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Errors the driver returns: ErrNotConfigured without an API key,
ErrRejected when the API refuses the request, ErrUnavailable when it
fails or answers something unreadable.
*/
var (
	ErrNotConfigured = errors.New("typesafe: no api key")
	ErrRejected      = errors.New("typesafe: request rejected")
	ErrUnavailable   = errors.New("typesafe: unavailable")
)

/*
Config is one account: its API key, the API's base URL, the model or
alias asked for, and the timeout of one call.
*/
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

/*
Driver is the System One client.
*/
type Driver struct {
	http  *httpclient.Client
	clock clock.Clock
	cfg   Config
}

/*
New returns the driver for an account.
*/
func New(http *httpclient.Client, clk clock.Clock, cfg Config) (*Driver, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, ErrNotConfigured
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.typesafe.ai/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "jev-latest"
	}
	return &Driver{http: http, clock: clk, cfg: cfg}, nil
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
	Instructions string `json:"instructions,omitempty"`
	Criteria     any    `json:"criteria,omitempty"`
}

/*
wireResponse is the API's answer.
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
Decide asks the questions in one request.
*/
func (d *Driver) Decide(ctx context.Context, req contracts.DecisionRequest) (*contracts.DecisionResponse, error) {
	body := wireRequest{Model: d.cfg.Model, State: req.State, Questions: map[string]wireQuestion{}}
	for name, q := range req.Questions {
		body.Questions[name] = toWire(q)
	}

	started := d.clock.Now()
	call := d.http.New().WithToken(d.cfg.APIKey).Accept("application/json")
	if d.cfg.Timeout > 0 {
		call = call.Timeout(d.cfg.Timeout)
	}
	resp, err := call.Post(ctx, strings.TrimSuffix(d.cfg.BaseURL, "/")+"/systemone", body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if resp.Status == 400 || resp.Status == 401 || resp.Status == 403 || resp.Status == 422 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrRejected, resp.Status, truncate(string(resp.Body)))
	}
	if resp.Status < 200 || resp.Status > 299 {
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.Status)
	}

	var out wireResponse
	if err := resp.JSON(&out); err != nil || len(out.Answers) == 0 {
		return nil, fmt.Errorf("%w: unreadable answer", ErrUnavailable)
	}
	return fromWire(out, d.clock.Now().Sub(started)), nil
}

/*
toWire maps one question to the API's shape.
*/
func toWire(q contracts.DecisionQuestion) wireQuestion {
	w := wireQuestion{Type: string(q.Type), Instructions: q.Instructions}
	switch q.Type {
	case contracts.DecisionChoice:
		w.Criteria = q.Choices
	case contracts.DecisionScore:
		w.Criteria = q.Levels
	case contracts.DecisionNoul:
		if q.True != "" || q.False != "" {
			w.Criteria = map[string]string{"true": q.True, "false": q.False}
		}
	}
	return w
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
truncate keeps an error body short.
*/
func truncate(s string) string {
	if len(s) > 300 {
		return s[:300]
	}
	return s
}
