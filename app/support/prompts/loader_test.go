package prompts

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
tree builds a prompt version with every required file in every family's
folder, overridden by the given files.
*/
func tree(files map[string]string) fstest.MapFS {
	version := "v9"
	out := fstest.MapFS{}
	for _, family := range Families() {
		for name := range Variables() {
			out["agent/"+version+"/"+string(family)+"/"+name] = &fstest.MapFile{Data: []byte("text " + string(family))}
		}
		out["agent/"+version+"/"+string(family)+"/defaults.md"] = &fstest.MapFile{Data: []byte(
			"unknown: u " + string(family) + "\nhandoff: h\nhandoff_later: l\nhandoff_unavailable: x\nmax_replies: m\nerror: e\nrefusal: r\n",
		)}
	}
	for path, data := range files {
		if data == "" {
			delete(out, "agent/"+version+"/"+path)
			continue
		}
		out["agent/"+version+"/"+path] = &fstest.MapFile{Data: []byte(data)}
	}
	return out
}

func TestLoader(t *testing.T) {
	t.Parallel()

	t.Run("loads the shipped tree with v2 as the default", func(t *testing.T) {
		t.Parallel()
		l, err := NewLoader("", nil)
		require.NoError(t, err)
		assert.Equal(t, "v2", l.Version(""))
		assert.Equal(t, "v2", l.Version("v1"))
		for _, family := range Families() {
			for _, key := range DefaultKeys() {
				assert.NotEmpty(t, l.Defaults("v2", family)[key], key)
			}
		}
	})

	t.Run("renders every answer variable", func(t *testing.T) {
		t.Parallel()
		l, err := NewLoader("", nil)
		require.NoError(t, err)
		file, err := l.Render("v2", enums.AIModelFamilyAnthropic, "answer.system.md", Vars{
			"agent_name": "Ada", "product_name": "Lovelace", "tone": "Warm.", "language": "German",
			"guidance": "Be brief.", "channel": "chat", "handoff": "available",
		})
		require.NoError(t, err)
		assert.Contains(t, file.Content, "Lovelace")
		assert.NotContains(t, file.Content, "{{")
		assert.Equal(t, "agent/v2/anthropic/answer.system.md", file.Path)
	})

	t.Run("refuses a version that is not compiled in", func(t *testing.T) {
		t.Parallel()
		_, err := NewLoader("v0", nil)
		assert.ErrorIs(t, err, ErrInvalid)
	})

	t.Run("refuses a tree missing a required file", func(t *testing.T) {
		t.Parallel()
		_, err := loadSet([]fs.FS{tree(map[string]string{"openweight/tone_concise.md": ""})}, "v9")
		assert.ErrorIs(t, err, ErrInvalid)
	})

	t.Run("refuses a file naming an unknown variable", func(t *testing.T) {
		t.Parallel()
		_, err := loadSet([]fs.FS{tree(map[string]string{
			"openai/answer.system.md": "Hello {{customer_secret}}",
		})}, "v9")
		assert.ErrorIs(t, err, ErrInvalid)
	})

	t.Run("refuses defaults missing a key", func(t *testing.T) {
		t.Parallel()
		_, err := loadSet([]fs.FS{tree(map[string]string{"anthropic/defaults.md": "unknown: u\n"})}, "v9")
		assert.ErrorIs(t, err, ErrInvalid)
	})

	t.Run("refuses a folder that is not a model family", func(t *testing.T) {
		t.Parallel()
		_, err := loadSet([]fs.FS{tree(map[string]string{"common/answer.system.md": "shared"})}, "v9")
		assert.ErrorIs(t, err, ErrInvalid)
	})

	t.Run("gives each family only its own files and lines", func(t *testing.T) {
		t.Parallel()
		s, err := loadSet([]fs.FS{tree(nil)}, "v9")
		require.NoError(t, err)
		l := &Loader{version: "v9", versions: map[string]*set{"v9": s}}

		for _, family := range Families() {
			file, err := l.GetVersion("v9", family, "answer.system.md")
			require.NoError(t, err)
			assert.Equal(t, "text "+string(family), file.Content)
			assert.Equal(t, "u "+string(family), l.Defaults("v9", family)["unknown"])
		}
	})
}
