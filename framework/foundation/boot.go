/*
|--------------------------------------------------------------------------
| Boot: Observability, Data and Keys
|--------------------------------------------------------------------------
|
| The first services up are the ones every later step reports through:
| the logger, telemetry and the clock. Then the stores the application
| cannot run without, PostgreSQL and Valkey, each checked at once so a
| bad address stops the boot instead of the first request. Pending
| migrations run next, before anything reads a table, and then the keys
| everything encrypted depends on.
|
*/

package foundation

import (
	"cmp"
	"context"
	"fmt"

	"github.com/exaring/otelpgx"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/telemetry"
)

/*
bootObservability starts the logger, telemetry and clock, and sets the
runtime's memory limit.
*/
func (a *Application) bootObservability(ctx context.Context) error {
	logOptions := a.Config.Log
	logOptions.Writer = a.opts.LogWriter

	logger, err := log.New(logOptions)
	if err != nil {
		return fmt.Errorf("start logger: %w", err)
	}

	a.Log = logger
	a.Clock = cmp.Or[clock.Clock](a.opts.Clock, clock.System())

	a.Telemetry, err = telemetry.New(ctx, a.Config.Telemetry, a.opts.Telemetry...)
	if err != nil {
		return fmt.Errorf("start telemetry: %w", err)
	}

	a.onClose("telemetry", a.Telemetry.Shutdown)
	applyMemoryLimit(ctx, a.Config.Runtime, a.Log)

	return nil
}

/*
bootData opens the database pool, traced like every request, applies
pending migrations when configured, and connects to Valkey.
*/
func (a *Application) bootData(ctx context.Context) error {
	dbConfig := a.Config.Database
	dbConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithTracerProvider(a.Telemetry.TracerProvider()),
		otelpgx.WithMeterProvider(a.Telemetry.MeterProvider()),
	)

	db, err := database.Open(ctx, dbConfig)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	a.DB = db
	a.onClose("database", closeFunc(db.Close))

	a.Migrator, err = newMigrator(a.Config, a.opts.Migrations, a.Log)
	if err != nil {
		return err
	}

	if a.Config.Migrations.Auto {
		if _, err := a.Migrator.Up(ctx); err != nil {
			return fmt.Errorf("migrate on boot: %w", err)
		}
	}

	a.Redis, err = redis.Open(ctx, a.Config.Redis)
	if err != nil {
		return fmt.Errorf("open valkey: %w", err)
	}

	a.onClose("valkey", closeFunc(a.Redis.Close))

	return nil
}

/*
bootSecurity resolves the app key and builds the encrypter, the key
encryption keys and the password hashers.
*/
func (a *Application) bootSecurity(ctx context.Context) error {
	key, err := resolveAppKey(ctx, a.Config.App, a.Log)
	if err != nil {
		return err
	}

	a.appKey = key

	a.Encrypter, err = encryption.NewEncrypter(key)
	if err != nil {
		return fmt.Errorf("build encrypter: %w", err)
	}

	a.Crypt = encryption.NewManager(a.Config.Encryption)
	a.Hash = hashing.NewManager(a.Config.Hashing)

	return nil
}
