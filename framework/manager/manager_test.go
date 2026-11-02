package manager_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
)

type disk struct{ name, root string }

func newManager() *manager.Manager[*disk] {
	m := manager.New[*disk]("disk", map[string]config.Section{
		"public":  {"driver": "local", "root": "/srv/public"},
		"private": {"driver": "local", "root": "/srv/private"},
		"remote":  {"driver": "s3"},
		"broken":  {"driver": "failing"},
	}, "public")

	m.Extend("local", func(name string, section config.Section) (*disk, error) {
		return &disk{name: name, root: section.String("root")}, nil
	})
	m.Extend("failing", func(string, config.Section) (*disk, error) {
		return nil, errors.New("no credentials")
	})

	return m
}

func TestManager(t *testing.T) {
	t.Parallel()

	t.Run("builds an instance once from its own section", func(t *testing.T) {
		t.Parallel()
		m := newManager()

		first, err := m.Instance("private")
		require.NoError(t, err)
		again, err := m.Instance("private")
		require.NoError(t, err)

		assert.Same(t, first, again)
		assert.Equal(t, &disk{name: "private", root: "/srv/private"}, first)
	})

	t.Run("resolves the default instance", func(t *testing.T) {
		t.Parallel()
		m := newManager()

		def, err := m.Default()

		require.NoError(t, err)
		assert.Equal(t, "public", def.name)
		assert.Equal(t, "public", m.DefaultName())
		assert.ElementsMatch(t, []string{"public", "private", "remote", "broken"}, m.Names())
	})

	t.Run("names what is missing or failing", func(t *testing.T) {
		t.Parallel()
		m := newManager()

		_, err := m.Instance("nope")
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
		require.ErrorContains(t, err, `disk "nope"`)

		_, err = m.Instance("remote")
		require.ErrorIs(t, err, manager.ErrUnknownDriver)
		require.ErrorContains(t, err, `driver "s3"`)

		_, err = m.Instance("broken")
		require.ErrorContains(t, err, `build disk "broken": no credentials`)
	})

	t.Run("serves a swapped instance to every caller", func(t *testing.T) {
		t.Parallel()
		m := newManager()
		fake := &disk{name: "fake"}

		m.Swap("remote", fake)

		got, err := m.Instance("remote")
		require.NoError(t, err)
		assert.Same(t, fake, got)
	})
}
