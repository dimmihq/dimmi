package foundation_test

import (
	"context"
	"errors"
	"math"
	"runtime/debug"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
driverEnv configures one named instance of every built-in driver.
*/
func driverEnv(t *testing.T) []string {
	t.Helper()

	return []string{
		"APP_MIGRATIONS__AUTO=false",
		"APP_AWS__REGION=us-east-1",
		"APP_AWS__ENDPOINT=" + testkit.Env(t, "AWS_ENDPOINT_URL_S3"),
		"APP_FILESYSTEMS__DISKS__LOCAL__DRIVER=local",
		"APP_FILESYSTEMS__DISKS__LOCAL__ROOT=" + t.TempDir(),
		"APP_FILESYSTEMS__DISKS__BUCKET__DRIVER=s3",
		"APP_FILESYSTEMS__DISKS__BUCKET__BUCKET=" + testkit.NewBucket(t),
		"APP_FILESYSTEMS__DISKS__BUCKET__FORCE_PATH_STYLE=true",
		"APP_FILESYSTEMS__DISKS__BUCKET__ENDPOINT=" + testkit.Env(t, "AWS_ENDPOINT_URL_S3"),
		"APP_MALWARE__SCANNERS__NONE__DRIVER=none",
		"APP_MALWARE__SCANNERS__CLAMAV__DRIVER=clamav",
		"APP_MALWARE__SCANNERS__GUARDDUTY__DRIVER=guardduty",
		"APP_MALWARE__SCANNERS__GUARDDUTY__BUCKET=uploads",
		"APP_MAIL__MAILERS__LOG__DRIVER=log",
		"APP_MAIL__MAILERS__SES__DRIVER=ses",
		"APP_MAIL__MAILERS__POSTMARK__DRIVER=postmark",
		"APP_MAIL__MAILERS__RESEND__DRIVER=resend",
		"APP_MAIL__MAILERS__MAILGUN__DRIVER=mailgun",
		"APP_MAIL__MAILERS__SENDGRID__DRIVER=sendgrid",
		"APP_MAILBOX__SOURCES__SES__DRIVER=ses",
		"APP_MAILBOX__SOURCES__SES__DISK=local",
		"APP_NOTIFICATIONS__CHANNELS__MAIL__DRIVER=mail",
		"APP_NOTIFICATIONS__CHANNELS__DATABASE__DRIVER=database",
		"APP_NOTIFICATIONS__CHANNELS__BROADCAST__DRIVER=broadcast",
		"APP_NOTIFICATIONS__CHANNELS__WEBPUSH__DRIVER=webpush",
		"APP_ENCRYPTION__KEKS__LOCAL__DRIVER=local",
		"APP_ENCRYPTION__KEKS__KMS__DRIVER=awskms",
		"APP_ENCRYPTION__KEKS__KMS__KEY_ARN=arn:aws:kms:us-east-1:000000000000:key/test",
		"APP_SOCIALITE__PROVIDERS__GITHUB__DRIVER=github",
		"APP_SOCIALITE__PROVIDERS__GOOGLE__DRIVER=google",
		"APP_SOCIALITE__PROVIDERS__OIDC__DRIVER=oidc",
		"APP_CUSTOM_DOMAINS__TLS__NONE__DRIVER=none",
		"APP_CUSTOM_DOMAINS__TLS__CADDYASK__DRIVER=caddyask",
		"APP_CUSTOM_DOMAINS__TLS__CERTMAGIC__DRIVER=certmagic",
		"APP_CUSTOM_DOMAINS__TLS__CERTMAGIC__EMAIL=ops@example.com",
	}
}

/*
registered fails the test when the instance's driver is not registered.
Other build errors, such as a missing credential, are allowed: only the
registration is under test.
*/
func registered(t *testing.T, name string, err error) {
	t.Helper()

	assert.False(t, errors.Is(err, manager.ErrUnknownDriver), "%s: %v", name, err)
}

func TestDrivers(t *testing.T) {
	t.Parallel()

	t.Run("registers every built-in driver with its manager", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, driverEnv(t)...)))

		lookups := map[string]func(name string) error{
			"disk":     func(name string) error { _, err := app.Storage.Disk(name); return err },
			"scanner":  func(name string) error { _, err := app.Scanner.Scanner(name); return err },
			"mailer":   func(name string) error { _, err := app.Mail.Mailer(name); return err },
			"mailbox":  func(name string) error { _, err := app.Mailbox.Source(name); return err },
			"channel":  func(name string) error { _, err := app.Notify.Channel(name); return err },
			"hasher":   func(name string) error { _, err := app.Hash.Hasher(name); return err },
			"kek":      func(name string) error { _, err := app.Crypt.KEK(name); return err },
			"social":   func(name string) error { _, err := app.Social.Driver(name); return err },
			"tls":      func(name string) error { _, err := app.Domains.Setup(name); return err },
			"detector": func(name string) error { _, err := app.Languages.Detector(name); return err },
		}
		instances := map[string][]string{
			"disk":     {"local", "bucket"},
			"scanner":  {"none", "clamav", "guardduty"},
			"mailer":   {"system", "log", "ses", "postmark", "resend", "mailgun", "sendgrid"},
			"mailbox":  {"ses"},
			"channel":  {"mail", "database", "broadcast", "webpush"},
			"hasher":   {"argon2id", "bcrypt"},
			"kek":      {"local", "kms"},
			"social":   {"github", "google", "oidc"},
			"tls":      {"none", "caddyask", "certmagic"},
			"detector": {"none", "whatlang"},
		}

		for kind, names := range instances {
			for _, name := range names {
				registered(t, kind+" "+name, lookups[kind](name))
			}
		}
	})

	t.Run("refuses every hostname without an authorizer and asks the one given", func(t *testing.T) {
		t.Parallel()
		refusing := newApp(t, options(environ(t, "APP_MIGRATIONS__AUTO=false")))
		require.Error(t, refusing.Domains.Ensure(t.Context(), "docs.example.com"))

		opts := options(environ(t, "APP_MIGRATIONS__AUTO=false"))
		opts.Domains = func(context.Context, string) error { return nil }
		allowing := newApp(t, opts)
		require.NoError(t, allowing.Domains.Ensure(t.Context(), "docs.example.com"))
	})

	t.Run("names every built-in middleware for routes", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_MIGRATIONS__AUTO=false")))

		for _, entry := range []string{"session", "auth", "can:view", "signed", "csrf", "idempotent"} {
			_, err := app.Kernel.Resolve(entry)
			require.NoError(t, err, entry)
		}

		ok, err := app.Gate.Can(t.Context(), nil, "view", nil)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("trusts the proxies it is given and refuses ones it cannot read", func(t *testing.T) {
		t.Parallel()
		newApp(t, options(environ(t, "APP_MIGRATIONS__AUTO=false", "APP_HTTP__TRUSTED_PROXIES=10.0.0.0/8")))

		_, err := foundation.New(t.Context(), options(environ(t,
			"APP_MIGRATIONS__AUTO=false", "APP_HTTP__TRUSTED_PROXIES=not-an-address")))

		require.ErrorContains(t, err, "trust proxies")
	})

	t.Run("translates with the application's files", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_MIGRATIONS__AUTO=false"))
		opts.Language = fstest.MapFS{"en/messages.json": {Data: []byte(`{"greeting": "Hello"}`)}}
		app := newApp(t, opts)

		assert.Equal(t, "Hello", app.Lang.T(i18n.WithLocale(t.Context(), "en"), "messages.greeting", nil))

		opts.Language = fstest.MapFS{"en/messages.json": {Data: []byte(`{`)}}
		_, err := foundation.New(t.Context(), opts)
		require.ErrorContains(t, err, "load translations")
	})

	t.Run("creates the table of every database notification channel", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t,
			"APP_NOTIFICATIONS__CHANNELS__ALERTS__DRIVER=database",
			"APP_NOTIFICATIONS__CHANNELS__ALERTS__TABLE=alerts",
			"APP_NOTIFICATIONS__CHANNELS__PUSH__DRIVER=webpush",
		)))

		var exists bool
		require.NoError(t, app.DB.QueryRow(t.Context(), "SELECT to_regclass('alerts') IS NOT NULL").Scan(&exists))
		assert.True(t, exists)
	})
}

func TestRuntime(t *testing.T) {
	t.Run("sets the memory limit it is given and reads the container's on auto", func(t *testing.T) {
		previous := debug.SetMemoryLimit(-1)
		t.Cleanup(func() { debug.SetMemoryLimit(previous) })

		newApp(t, options(environ(t, "APP_MIGRATIONS__AUTO=false", "APP_RUNTIME__MEMORY_LIMIT=4GiB")))
		assert.Equal(t, int64(4<<30), debug.SetMemoryLimit(-1))

		debug.SetMemoryLimit(math.MaxInt64)
		newApp(t, options(environ(t, "APP_MIGRATIONS__AUTO=false", "APP_RUNTIME__MEMORY_LIMIT=auto")))
	})
}
