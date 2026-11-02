/*
|--------------------------------------------------------------------------
| Runtime Limits
|--------------------------------------------------------------------------
|
| A container that exceeds its memory limit is killed without warning.
| Telling the Go runtime the limit makes the garbage collector work
| harder as the process nears it, instead of growing until the kernel
| steps in. The limit is read from the container's cgroup, set to a
| fixed size, or left alone.
|
*/

package foundation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/KimMachineGun/automemlimit/memlimit"

	"github.com/dimmihq/dimmi/framework/log"
)

const (
	memoryAuto  = "auto"
	memoryOff   = "off"
	memoryRatio = 0.9
)

/*
RuntimeConfig tunes the Go runtime. MemoryLimit is auto, which sets
the limit to nine tenths of the container's memory when it has a limit;
off, which leaves the runtime alone; or a size in bytes, optionally with
a KiB, MiB or GiB suffix.
*/
type RuntimeConfig struct {
	MemoryLimit string `default:"auto" koanf:"memory_limit"`
}

/*
checkMemoryLimit accepts auto, off or a size.
*/
func checkMemoryLimit(value string) error {
	if value == memoryAuto || value == memoryOff {
		return nil
	}

	if _, err := parseSize(value); err != nil {
		return invalid("runtime.memory_limit", err)
	}

	return nil
}

/*
parseSize reads a positive byte count with an optional binary suffix.
*/
func parseSize(value string) (uint64, error) {
	multiplier := uint64(1)
	for suffix, factor := range map[string]uint64{"KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30} {
		if trimmed, ok := strings.CutSuffix(value, suffix); ok {
			value, multiplier = trimmed, factor

			break
		}
	}

	n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("%q is not auto, off or a size such as 512MiB", value)
	}

	return n * multiplier, nil
}

/*
applyMemoryLimit sets the runtime's memory limit as configured. Not
finding a container limit is no reason to stop, so that is only logged.
*/
func applyMemoryLimit(ctx context.Context, cfg RuntimeConfig, logger log.Logger) {
	if cfg.MemoryLimit == memoryOff {
		return
	}

	provider := memlimit.ApplyRatio(memlimit.FromCgroup, memoryRatio)
	if cfg.MemoryLimit != memoryAuto {
		size, _ := parseSize(cfg.MemoryLimit)
		provider = memlimit.Limit(size)
	}

	limit, err := memlimit.Set(memlimit.WithProvider(provider), memlimit.WithRatio(1))
	if err != nil && !errors.Is(err, memlimit.ErrNoLimit) {
		logger.WarnContext(ctx, "memory limit not set", log.Err(err))

		return
	}

	logger.DebugContext(ctx, "memory limit set", log.Int64("bytes", limit))
}
