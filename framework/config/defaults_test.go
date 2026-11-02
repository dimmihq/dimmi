package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
)

type mode string

type limits struct {
	Burst uint16 `default:"20" koanf:"burst"`
}

type kinds struct {
	Name     string        `default:"app"         koanf:"name"`
	Mode     mode          `default:"auto"        koanf:"mode"`
	Debug    bool          `default:"true"        koanf:"debug"`
	Int      int           `default:"-7"          koanf:"int"`
	Int8     int8          `default:"8"           koanf:"int8"`
	Int16    int16         `default:"16"          koanf:"int16"`
	Int32    int32         `default:"32"          koanf:"int32"`
	Int64    int64         `default:"0x40"        koanf:"int64"`
	Uint     uint          `default:"1"           koanf:"uint"`
	Uint8    uint8         `default:"255"         koanf:"uint8"`
	Uint32   uint32        `default:"32"          koanf:"uint32"`
	Uint64   uint64        `default:"64"          koanf:"uint64"`
	Float32  float32       `default:"0.5"         koanf:"float32"`
	Float64  float64       `default:"2.25"        koanf:"float64"`
	Timeout  time.Duration `default:"1m30s"       koanf:"timeout"`
	Locales  []string      `default:"en, fr,,de"  koanf:"locales"`
	Ports    []int         `default:"80,443"      koanf:"ports"`
	Roles    []mode        `default:"[\"web\"]"   koanf:"roles"`
	Limits   limits        `koanf:"limits"`
	Optional *limits       `koanf:"optional"`
	Required *limits       `default:"{}"          koanf:"required"`
	Headers  map[string]string
	Skipped  string `default:"-"`
	Plain    string
	hidden   string `default:"never"`
}

func TestDefaults(t *testing.T) {
	t.Parallel()

	t.Run("fills every supported kind", func(t *testing.T) {
		t.Parallel()

		var got kinds
		require.NoError(t, config.Section{}.Decode(&got))

		assert.Equal(t, kinds{
			Name: "app", Mode: "auto", Debug: true,
			Int: -7, Int8: 8, Int16: 16, Int32: 32, Int64: 64,
			Uint: 1, Uint8: 255, Uint32: 32, Uint64: 64,
			Float32: 0.5, Float64: 2.25,
			Timeout:  90 * time.Second,
			Locales:  []string{"en", "fr", "de"},
			Ports:    []int{80, 443},
			Roles:    []mode{"web"},
			Limits:   limits{Burst: 20},
			Required: &limits{Burst: 20},
		}, got)
		assert.Empty(t, got.hidden)
	})

	t.Run("keeps fields that are already set", func(t *testing.T) {
		t.Parallel()
		got := kinds{
			Name: "set", Int: 1, Timeout: time.Second, Locales: []string{"es"},
			Limits: limits{Burst: 3}, Optional: &limits{}, Headers: map[string]string{"a": "b"},
		}

		require.NoError(t, config.Section{}.Decode(&got))

		assert.Equal(t, "set", got.Name)
		assert.Equal(t, 1, got.Int)
		assert.Equal(t, time.Second, got.Timeout)
		assert.Equal(t, []string{"es"}, got.Locales)
		assert.Equal(t, uint16(3), got.Limits.Burst)
		assert.Equal(t, &limits{Burst: 20}, got.Optional)
		assert.Equal(t, map[string]string{"a": "b"}, got.Headers)
	})

	t.Run("reports a default that does not parse", func(t *testing.T) {
		t.Parallel()

		for name, target := range map[string]any{
			"bool": &struct {
				B bool `default:"maybe"`
			}{},
			"int": &struct {
				I int8 `default:"300"`
			}{},
			"uint": &struct {
				U uint `default:"-1"`
			}{},
			"float": &struct {
				F float64 `default:"x"`
			}{},
			"duration": &struct {
				D time.Duration `default:"soon"`
			}{},
			"json list": &struct {
				L []string `default:"[1"`
			}{},
			"list item": &struct {
				L []int `default:"1,x"`
			}{},
			"kind": &struct {
				C complex64 `default:"1"`
			}{},
			"nested": &struct {
				N struct {
					I int `default:"x"`
				}
			}{},
		} {
			require.ErrorContains(t, config.Section{}.Decode(target), "apply defaults", name)
		}
	})

	t.Run("needs a pointer to a struct", func(t *testing.T) {
		t.Parallel()

		var nilPointer *kinds
		require.ErrorContains(t, config.Section{}.Decode(nilPointer), "apply defaults")
		require.ErrorContains(t, config.Section{}.Decode(kinds{}), "apply defaults")
	})
}
