/*
|--------------------------------------------------------------------------
| Fake Embedder
|--------------------------------------------------------------------------
|
| Tests decide which texts mean the same thing without calling a
| provider: a text containing a scripted phrase gets that phrase's
| vector, so a question and the article that answers it can be put
| side by side while sharing no words. Every other text gets a vector
| derived from its hash, the same on every run and almost orthogonal
| to all others. Errors and hangs are scripted the same way, and every
| request is kept for the test to read back. Outside a test binary
| every call fails.
|
*/

package fake

import (
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/dimmihq/dimmi/framework/hashing"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/openaicompat"
)

/*
ErrOutsideTests is what every call answers outside a test binary.
*/
var ErrOutsideTests = errors.New("fake embedder: only available in tests")

/*
phrase is one scripted phrase and its vector.
*/
type phrase struct {
	text   string
	vector []float32
}

/*
Embedder is a scripted contracts.Embeddings. Dims is the size it
answers at when a request asks for none.
*/
type Embedder struct {
	mu       sync.Mutex
	provider string
	dims     int
	phrases  []phrase
	err      error
	hang     bool
	sent     []contracts.EmbedRequest
}

/*
New returns a fake for the provider family, at the native size.
*/
func New(provider string, dims int) *Embedder {
	return &Embedder{provider: provider, dims: dims}
}

/*
Script gives every text containing the phrase, ignoring case, the
vector; earlier phrases win.
*/
func (e *Embedder) Script(text string, vector []float32) *Embedder {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.phrases = append(e.phrases, phrase{text: strings.ToLower(text), vector: vector})
	return e
}

/*
Fail makes every call answer the error, nil to answer again.
*/
func (e *Embedder) Fail(err error) *Embedder {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.err = err
	return e
}

/*
Hang makes every call wait until its context ends, as a provider that
never answers.
*/
func (e *Embedder) Hang() *Embedder {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hang = true
	return e
}

/*
Sent returns the requests received, in order.
*/
func (e *Embedder) Sent() []contracts.EmbedRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]contracts.EmbedRequest(nil), e.sent...)
}

/*
Provider names the provider family the fake stands in for.
*/
func (e *Embedder) Provider() string {
	return e.provider
}

/*
Models lists one model at the fake's size.
*/
func (e *Embedder) Models(context.Context) ([]contracts.EmbeddingModel, error) {
	if !testing.Testing() {
		return nil, ErrOutsideTests
	}
	return []contracts.EmbeddingModel{{ID: "fake-embed", NativeDims: e.dims, MaxBatch: contracts.MaxEmbedBatch}}, nil
}

/*
Embed answers the scripted vectors at the requested size.
*/
func (e *Embedder) Embed(ctx context.Context, req contracts.EmbedRequest) (*contracts.EmbedResponse, error) {
	if !testing.Testing() {
		return nil, ErrOutsideTests
	}
	e.mu.Lock()
	e.sent = append(e.sent, req)
	err, hang := e.err, e.hang
	e.mu.Unlock()
	if hang {
		<-ctx.Done()
		return nil, &contracts.LLMError{Kind: contracts.LLMErrorTimeout, Message: "fake embedder timed out"}
	}
	if err != nil {
		return nil, err
	}
	dims := req.Dimensions
	if dims == 0 {
		dims = e.dims
	}
	out := &contracts.EmbedResponse{Dims: dims, Model: req.Model, Usage: contracts.EmbedUsage{Reported: true}}
	for _, in := range req.Inputs {
		v, err := openaicompat.Normalize(fit(e.vectorFor(in), dims))
		if err != nil {
			return nil, err
		}
		out.Vectors = append(out.Vectors, v)
		out.Usage.InputTokens += int64(len(strings.Fields(in)))
	}
	return out, nil
}

/*
vectorFor is the scripted vector of the first phrase the text holds,
else the text's own hashed vector.
*/
func (e *Embedder) vectorFor(text string) []float32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	lower := strings.ToLower(text)
	for _, p := range e.phrases {
		if strings.Contains(lower, p.text) {
			return p.vector
		}
	}
	return hashed(text, max(e.dims, 1))
}

/*
Axis is the unit vector along one axis, for scripting texts that mean
the same thing (one axis) or nothing alike (two axes).
*/
func Axis(i, dims int) []float32 {
	v := make([]float32, dims)
	v[i%dims] = 1
	return v
}

/*
hashed spreads the text's hash over the vector, in -1..1.
*/
func hashed(text string, dims int) []float32 {
	v := make([]float32, dims)
	seed := hashing.SHA256([]byte(text))
	for i := range v {
		block := hashing.SHA256(append(append([]byte(nil), seed...), byte(i), byte(i>>8)))
		v[i] = float32(binary.BigEndian.Uint32(block[:4]))/float32(1<<31) - 1
	}
	return v
}

/*
fit pads or cuts a vector to the size.
*/
func fit(v []float32, dims int) []float32 {
	out := make([]float32, dims)
	copy(out, v)
	return out
}
