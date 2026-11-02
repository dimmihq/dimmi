package validation_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
broken returns the rule each field broke, keyed by field path, or nil
when the value passes.
*/
func broken(t *testing.T, factory *validation.Factory, value any) map[string]string {
	t.Helper()

	err := factory.Struct(t.Context(), value)
	if err == nil {
		return nil
	}

	rules := map[string]string{}
	for _, v := range violations(t, err).Violations {
		rules[v.Field] = v.Rule
	}

	return rules
}

type numbers struct {
	Count   int     `json:"count"   validate:"gte=1,lte=10"`
	Small   int8    `json:"small"   validate:"gt=0,lt=5"`
	Size    uint    `json:"size"    validate:"min=2,max=4"`
	Ratio   float64 `json:"ratio"   validate:"gt=0.5"`
	Exact   string  `json:"exact"   validate:"len=3"`
	Items   []int   `json:"items"   validate:"len=2"`
	Mode    string  `json:"mode"    validate:"oneof=fast slow"`
	Level   int     `json:"level"   validate:"oneof=1 2 3"`
	Kind    string  `json:"kind"    validate:"eq=user"`
	Status  string  `json:"status"  validate:"ne=banned"`
	Enabled bool    `json:"enabled" validate:"eq=true"`
	Answer  int     `json:"answer"  validate:"ne=0"`
}

type formats struct {
	Email    string `json:"email"    validate:"email"`
	Link     string `json:"link"     validate:"url"`
	ID       string `json:"id"       validate:"uuid"`
	Letters  string `json:"letters"  validate:"alpha"`
	Handle   string `json:"handle"   validate:"alphanum"`
	Optional string `json:"optional" validate:"omitempty,email"`
}

func TestRules(t *testing.T) {
	t.Parallel()

	t.Run("compares sizes, numbers and listed values", func(t *testing.T) {
		t.Parallel()
		factory := newFactory(t)
		valid := numbers{
			Count: 5, Small: 2, Size: 3, Ratio: 0.75, Exact: "abc", Items: []int{1, 2},
			Mode: "fast", Level: 2, Kind: "user", Status: "active", Enabled: true, Answer: 42,
		}

		assert.Nil(t, broken(t, factory, valid))
		assert.Equal(t, map[string]string{
			"count": "gte", "small": "gt", "size": "min", "ratio": "gt", "exact": "len", "items": "len",
			"mode": "oneof", "level": "oneof", "kind": "eq", "status": "ne", "enabled": "eq", "answer": "ne",
		}, broken(t, factory, numbers{Status: "banned", Exact: "ab"}))
		assert.Equal(t, map[string]string{"count": "lte", "small": "lt", "size": "max"},
			broken(t, factory, func() numbers { v := valid; v.Count, v.Small, v.Size = 11, 5, 5; return v }()))
	})

	t.Run("checks the format of text", func(t *testing.T) {
		t.Parallel()
		factory := newFactory(t)
		valid := formats{
			Email: "ana@example.com", Link: "https://example.com/a", ID: "123E4567-e89b-12d3-a456-426614174000",
			Letters: "Ana", Handle: "ana42",
		}

		assert.Nil(t, broken(t, factory, valid))
		assert.Nil(t, broken(t, factory, formats{
			Email: "ana@example.com", Link: "mailto:ana@example.com", ID: valid.ID, Letters: "a", Handle: "1",
		}))
		assert.Equal(t, map[string]string{
			"email": "email", "link": "url", "id": "uuid", "letters": "alpha", "handle": "alphanum",
			"optional": "email",
		}, broken(t, factory, formats{
			Email: "Ana <ana@example.com>", Link: "/relative", ID: "123e4567-e89b-12d3-a456-42661417400g",
			Letters: "ana1", Handle: "ana_42", Optional: "nope",
		}))
		assert.Equal(t, "uuid", broken(t, factory, formats{
			Email: valid.Email, Link: valid.Link, ID: "123e4567e89b-12d3-a456-4266141740000", Letters: "a", Handle: "a",
		})["id"])
		assert.Equal(t, "uuid", broken(t, factory, formats{
			Email: valid.Email, Link: valid.Link, ID: "short", Letters: "a", Handle: "a",
		})["id"])
	})

	t.Run("compares a time with the present of the clock", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
		factory, err := validation.New(validation.WithClock(clock.NewFake(now)))
		require.NoError(t, err)

		type window struct {
			Starts time.Time `json:"starts" validate:"gt"`
			Ends   time.Time `json:"ends"   validate:"lte"`
		}

		assert.Nil(t, broken(t, factory, window{Starts: now.Add(time.Hour), Ends: now}))
		assert.Equal(t, map[string]string{"starts": "gt", "ends": "lte"},
			broken(t, factory, window{Starts: now, Ends: now.Add(time.Second)}))
	})
}

type person struct {
	Name string `json:"name" validate:"required"`
}

type base struct {
	ID string `json:"id" validate:"required"`
}

type Embedded struct {
	Note string `json:"note" validate:"max=2"`
}

type shapes struct {
	base
	*Embedded

	People   []person        `json:"people" validate:"required,dive"`
	Owner    *person         `json:"owner"`
	Manager  *person         `json:"manager"  validate:"required"`
	Nickname *string         `json:"nickname" validate:"omitempty,min=1"`
	Title    *string         `json:"title"    validate:"max=3"`
	Flag     *bool           `json:"flag"     validate:"required"`
	Scores   map[string]int  `json:"scores"   validate:"dive,keys,alpha,endkeys,min=1"`
	Grid     [2][]string     `json:"grid"     validate:"dive,dive,alphanum"`
	Extra    any             `json:"extra"    validate:"required"`
	Ignored  string          `json:"ignored"  validate:"-"`
	Named    map[int]*person `json:"named"    validate:"dive"`
	private  string          `validate:"required"`
}

func TestWalk(t *testing.T) {
	t.Parallel()

	t.Run("walks nested values and names them by path", func(t *testing.T) {
		t.Parallel()
		empty, long, no := "", "long", false
		input := shapes{
			Embedded: &Embedded{Note: "long"},
			People:   []person{{Name: "Ana"}, {}},
			Owner:    &person{},
			Nickname: &empty,
			Title:    &long,
			Flag:     &no,
			Scores:   map[string]int{"b1": 1, "a": 0, "c": 2},
			Grid:     [2][]string{{"ok", "no!"}},
			Named:    map[int]*person{7: {}},
			private:  "private",
		}

		assert.Equal(t, map[string]string{
			"id": "required", "note": "max", "people.1.name": "required", "owner.name": "required",
			"manager": "required", "nickname": "min", "title": "max", "scores.b1": "alpha", "scores.a": "min",
			"grid.0.1": "alphanum", "extra": "required", "named.7.name": "required",
		}, broken(t, newFactory(t), &input))
		assert.Equal(t, "private", input.private)
	})

	t.Run("treats an empty list as missing", func(t *testing.T) {
		t.Parallel()

		type list struct {
			Tags []string `json:"tags" validate:"required"`
		}

		assert.Equal(t, map[string]string{"tags": "required"}, broken(t, newFactory(t), list{Tags: []string{}}))
	})

	t.Run("skips an embedded struct that is not set", func(t *testing.T) {
		t.Parallel()
		name, flag := "a", true
		input := shapes{
			base: base{ID: "x"}, People: []person{{Name: "Ana"}}, Manager: &person{Name: "Bo"},
			Nickname: &name, Flag: &flag, Extra: 1,
		}

		assert.Nil(t, broken(t, newFactory(t), input))
	})
}

type passing struct{ name string }

func (p passing) Name() string { return p.name }

func (passing) Message() string { return "never" }

func (passing) Passes(context.Context, any, string) bool { return true }

func TestRuleMistakes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		value any
		err   string
	}{
		"a rule nobody registered": {
			value: struct {
				A string `validate:"shiny"`
			}{},
			err: `field A: rule "shiny": validation: invalid rule: no rule named "shiny"`,
		},
		"a size that is not a number": {
			value: struct {
				A string `validate:"max=ten"`
			}{},
			err: `parameter "ten" is not a number`,
		},
		"an unsigned size that is not a number": {
			value: struct {
				A uint `validate:"max=-1"`
			}{},
			err: `parameter "-1" is not a number`,
		},
		"a float size that is not a number": {
			value: struct {
				A float32 `validate:"max=x"`
			}{},
			err: `parameter "x" is not a number`,
		},
		"a value that cannot be measured": {
			value: struct {
				A bool `validate:"max=1"`
			}{},
			err: "cannot measure a bool",
		},
		"a boolean compared with text": {
			value: struct {
				A bool `validate:"eq=yes"`
			}{},
			err: `parameter "yes" is not a boolean`,
		},
		"a format rule on a number": {
			value: struct {
				A int `validate:"email"`
			}{},
			err: "a int is not text",
		},
		"a dive into a scalar": {
			value: struct {
				A string `validate:"dive,required"`
			}{},
			err: "cannot dive into a string",
		},
		"keys without their end": {
			value: struct {
				A map[string]string `validate:"dive,keys,alpha"`
			}{},
			err: `"keys" without "endkeys"`,
		},
		"keys out of place": {
			value: struct {
				A string `validate:"required,keys"`
			}{},
			err: `"keys" out of place`,
		},
		"a dive among the keys": {
			value: struct {
				A map[string]string `validate:"dive,keys,dive,endkeys"`
			}{},
			err: "only rules may check keys",
		},
		"an empty rule": {
			value: struct {
				A string `validate:"required,,max=1"`
			}{},
			err: `"" out of place`,
		},
		"a broken rule on a key": {
			value: struct {
				A map[string]string `validate:"dive,keys,max=x,endkeys"`
			}{A: map[string]string{"k": "v"}},
			err: `field A.k: rule "max"`,
		},
		"a broken rule on an item": {
			value: struct {
				A []string `validate:"dive,max=x"`
			}{A: []string{"v"}},
			err: `field A.0: rule "max"`,
		},
		"a broken rule in a nested struct": {
			value: struct {
				A struct {
					B string `validate:"max=x"`
				}
			}{},
			err: `field A.B: rule "max"`,
		},
		"a broken rule in an embedded struct": {
			value: struct {
				Embedded
				B struct {
					C string `validate:"dive"`
				}
			}{Embedded: Embedded{Note: "x"}},
			err: "cannot dive into a string",
		},
		"a broken tag in an embedded struct": {
			value: struct {
				*Embedded
				B struct {
					C string `validate:"keys"`
				}
			}{Embedded: &Embedded{}},
			err: "field C:",
		},
		"a broken rule in a map item": {
			value: struct {
				A map[string][]string `validate:"dive,dive,max=x"`
			}{A: map[string][]string{"k": {"v"}}},
			err: `field A.k.0: rule "max"`,
		},
	}

	for name, tc := range cases {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()

			err := newFactory(t).Struct(t.Context(), tc.value)

			require.ErrorIs(t, err, validation.ErrInvalidRule)
			require.ErrorContains(t, err, tc.err)
		})
	}

	t.Run("refuses names that steer the walk", func(t *testing.T) {
		t.Parallel()
		factory := newFactory(t)

		for _, name := range []string{"dive", "omitempty", "keys", "endkeys", "a,b", "a=b"} {
			require.ErrorIs(t, factory.Register(passing{name: name}), validation.ErrInvalidRule, name)
		}
	})

	t.Run("lets a registered rule replace a built-in one", func(t *testing.T) {
		t.Parallel()
		factory := newFactory(t)
		require.NoError(t, factory.Register(passing{name: "email"}))

		assert.Nil(t, broken(t, factory, formats{
			Email: "nope", Link: "https://example.com", ID: "123e4567-e89b-12d3-a456-426614174000", Letters: "a", Handle: "a",
		}))
	})

	t.Run("refuses a nil pointer", func(t *testing.T) {
		t.Parallel()

		require.ErrorIs(t, newFactory(t).Struct(t.Context(), (*shapes)(nil)), validation.ErrNotStruct)
	})
}
