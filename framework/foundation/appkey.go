/*
|--------------------------------------------------------------------------
| App Key
|--------------------------------------------------------------------------
|
| The app key encrypts secrets and signs URLs, so losing it loses every
| encrypted value. It comes from configuration, or from a key file in the
| data directory. When neither exists and generating is allowed, a new
| key is written there exactly once: replicas sharing the directory race
| to create the file and the losers read the winner's key, so no two
| servers ever run with different keys.
|
*/

package foundation

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
ErrNoAppKey stops the boot when no app key is configured, none is stored
in the data directory and generating one is off.
*/
var ErrNoAppKey = errors.New("foundation: no app key; set app.key or turn on app.key_auto_generate")

const keyFileName = "app.key"

/*
resolveAppKey returns the app key bytes from configuration, the key
file, or a newly generated key file, in that order.
*/
func resolveAppKey(ctx context.Context, cfg AppConfig, logger log.Logger) ([]byte, error) {
	if cfg.Key != "" {
		return parseAppKey(cfg.Key, "app.key")
	}

	path := filepath.Join(cfg.DataDir, keyFileName)

	stored, err := os.ReadFile(filepath.Clean(path))
	if err == nil {
		return parseAppKey(strings.TrimSpace(string(stored)), path)
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read app key: %w", err)
	}

	if !cfg.KeyAutoGenerate {
		return nil, ErrNoAppKey
	}

	if err := writeAppKey(path); err != nil {
		return nil, err
	}

	logger.WarnContext(ctx, "generated a new app key; back it up apart from the database",
		log.String("path", path))

	return resolveAppKey(ctx, cfg, logger)
}

/*
writeAppKey creates the key file with a fresh key, readable by the owner
only. A file created meanwhile by another server is kept.
*/
func writeAppKey(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create app key directory: %w", err)
	}

	file, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("create app key: %w", err)
	}

	_, err = file.WriteString(encryption.GenerateAppKey() + "\n")

	return errors.Join(err, file.Close())
}

func parseAppKey(value, source string) ([]byte, error) {
	key, err := encryption.ParseAppKey(value)
	if err != nil {
		return nil, invalid(source, err)
	}

	return key, nil
}
