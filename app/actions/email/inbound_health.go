/*
|--------------------------------------------------------------------------
| Inbound Health
|--------------------------------------------------------------------------
|
| Some provider settings make every delivery useless while the provider
| keeps retrying, so nothing looks wrong until its retries run out. The
| last such signal is kept for the operator's checks to name the setting
| to change. It clears itself on the next good delivery, and losing it
| only loses a hint.
|
*/

package email

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
healthSignalLifetime is how long a signal stays without a new delivery.
*/
const healthSignalLifetime = 7 * 24 * time.Hour

/*
InboundHealth keeps the last misconfiguration signal of each source.
*/
type InboundHealth struct {
	Cache *cache.Store
	Log   log.Logger
}

/*
RecordMisconfigured keeps the setting the source's deliveries are
missing.
*/
func (h *InboundHealth) RecordMisconfigured(ctx context.Context, source, setting string) {
	err := h.Cache.Set(ctx, healthKey(source), []byte(setting), healthSignalLifetime)
	if err != nil {
		h.Log.WarnContext(ctx, "record inbound misconfiguration", log.String("source", source), log.Err(err))
	}
}

/*
ClearMisconfigured forgets the source's signal after a good delivery.
*/
func (h *InboundHealth) ClearMisconfigured(ctx context.Context, source string) {
	err := h.Cache.Delete(ctx, healthKey(source))
	if err != nil {
		h.Log.WarnContext(ctx, "clear inbound misconfiguration", log.String("source", source), log.Err(err))
	}
}

/*
Misconfigured returns the setting the source's last delivery was
missing, empty when there is none.
*/
func (h *InboundHealth) Misconfigured(ctx context.Context, source string) (string, error) {
	setting, err := h.Cache.Get(ctx, healthKey(source))
	if errors.Is(err, cache.ErrMiss) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return string(setting), nil
}

func healthKey(source string) string {
	return "email:inbound:misconfigured:" + source
}
