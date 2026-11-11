/*
|--------------------------------------------------------------------------
| Failover Driver
|--------------------------------------------------------------------------
|
| Some models are served by more than one host under the same names.
| The hosts are tried in order: a call goes to the first, and only when
| that host cannot serve right now (no credit left, throttled,
| overloaded, failing, slow or unreachable, or missing the model) does
| the same request move to the next. A request the caller got wrong is
| never sent again, since every host would refuse it the same way, and
| a stream moves only before it opens, because a half answer cannot be
| unsent. A host whose breaker is open is skipped without a call, so an
| outage costs one round trip per breaker window rather than one per
| request. Each move is logged with the host and failure kind only,
| never the key or the prompt.
|
*/

package failover

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/llmretry"
)

/*
Options configures the driver. Hosts are tried in order and must serve
the same model names; BreakerFor returns the breaker guarding one host
and model, or nil for none.
*/
type Options struct {
	Hosts      []contracts.LLM
	BreakerFor func(host contracts.LLMProvider, model string) *llmretry.Breaker
}

/*
Driver is a contracts.LLM over an ordered list of hosts.
*/
type Driver struct {
	opts Options
}

/*
New builds the driver over at least one host. A single host is
returned as it is, since there is nothing to move to.
*/
func New(o Options) contracts.LLM {
	if len(o.Hosts) == 1 {
		return o.Hosts[0]
	}

	return &Driver{opts: o}
}

/*
Provider names the first host, the one calls normally reach.
*/
func (d *Driver) Provider() contracts.LLMProvider { return d.opts.Hosts[0].Provider() }

/*
Chat runs the completion on the first host able to serve it.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	return run(ctx, d, req.Model, func(h contracts.LLM) (*contracts.ChatResponse, error) {
		return h.Chat(ctx, req)
	})
}

/*
Stream opens the stream on the first host able to serve it.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	return run(ctx, d, req.Model, func(h contracts.LLM) (contracts.ChatStream, error) {
		return h.Stream(ctx, req)
	})
}

/*
ListModels lists the models of the first host able to answer.
*/
func (d *Driver) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	return run(ctx, d, "", func(h contracts.LLM) ([]contracts.LLMModel, error) {
		return h.ListModels(ctx)
	})
}

/*
Capabilities answers from the first host. The hosts serve the same
models, so a request shaped for the first fits the others.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	return d.opts.Hosts[0].Capabilities(model)
}

/*
run tries each host in turn and returns the first answer, or the
failure of the last host tried. It stops early on a failure no other
host would avoid, or once the caller has given up.
*/
func run[T any](ctx context.Context, d *Driver, model string, call func(contracts.LLM) (T, error)) (T, error) {
	var zero T
	var last error
	hosts := d.opts.Hosts

	for i, host := range hosts {
		final := i == len(hosts)-1
		breaker := d.breaker(host, model)
		if !final && breaker != nil && breaker.Allow() != nil {
			continue
		}

		res, err := call(host)
		record(breaker, err)
		if err == nil {
			return res, nil
		}

		last = err
		if final || ctx.Err() != nil || !HostUnavailable(err) {
			return zero, err
		}
		warn(ctx, model, hosts[i:i+2], err)
	}

	return zero, last
}

/*
breaker returns the breaker of one host and model, nil when none is
configured.
*/
func (d *Driver) breaker(host contracts.LLM, model string) *llmretry.Breaker {
	if d.opts.BreakerFor == nil {
		return nil
	}

	return d.opts.BreakerFor(host.Provider(), model)
}

/*
record feeds one outcome to a host's breaker: a host that could not
serve counts against it, a cancellation says nothing, and any other
answer, a refused request included, proves the host is up.
*/
func record(b *llmretry.Breaker, err error) {
	if b == nil {
		return
	}

	llmErr, isLLM := errors.AsType[*contracts.LLMError](err)
	switch {
	case err == nil:
		b.Success()
	case isLLM && llmErr.Kind == contracts.LLMErrorCanceled:
		b.Release()
	case HostUnavailable(err):
		b.Failure()
	default:
		b.Success()
	}
}

/*
HostUnavailable reports whether a failure belongs to the host rather
than the request: its credit or quota is gone, its key is refused, it
throttles, is overloaded, fails, times out, cannot be reached or is
blocked, or does not serve the model. A bad request, filtered content
or a cancellation would fail the same way anywhere, and so does any
error that is not a provider failure, such as a schema the model
cannot enforce.
*/
func HostUnavailable(err error) bool {
	llmErr, ok := errors.AsType[*contracts.LLMError](err)
	if !ok {
		return false
	}

	switch llmErr.Kind {
	case contracts.LLMErrorQuota, contracts.LLMErrorAuth, contracts.LLMErrorPermission,
		contracts.LLMErrorRateLimited, contracts.LLMErrorOverloaded, contracts.LLMErrorServer,
		contracts.LLMErrorTimeout, contracts.LLMErrorNetwork, contracts.LLMErrorModelNotFound,
		contracts.LLMErrorBlockedHost:
		return true
	case contracts.LLMErrorInvalidRequest, contracts.LLMErrorContentFiltered, contracts.LLMErrorCanceled:
		return false
	}

	return false
}

/*
warn logs that a call moved from the first host of the pair to the
second, with the failure's kind and status but never its key or
prompt.
*/
func warn(ctx context.Context, model string, pair []contracts.LLM, err error) {
	attrs := []any{
		log.String("from", string(pair[0].Provider())), log.String("to", string(pair[1].Provider())),
		log.String("model", model),
	}
	if llmErr, ok := errors.AsType[*contracts.LLMError](err); ok {
		attrs = append(attrs, log.String("kind", string(llmErr.Kind)), log.Int("status", llmErr.HTTPStatus))
	}

	log.From(ctx).WarnContext(ctx, "llm host unavailable, trying the next", attrs...)
}
