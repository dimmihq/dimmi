/*
|--------------------------------------------------------------------------
| Configuration Loading
|--------------------------------------------------------------------------
|
| Configuration comes from layers, each overriding the one before: the
| struct's own defaults, the YAML files, the environment, secret files
| named by the environment, and settings stored at runtime. Operators can
| therefore ship one file and change a single value per deployment
| without editing it, and secrets never have to sit in a file or in the
| process environment. The application names its own file and prefix;
| nothing here knows which product is loading.
|
*/

package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"reflect"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

/*
LoadOptions says where configuration comes from.

Name names the configuration in errors and, when Files is empty, gives
the optional default file Name.yaml. Files lists YAML files loaded in
order, later ones winning; each must exist. EnvPrefix selects the
environment variables to read, such as APP for APP_MAIL__DEFAULT; when
it is empty the environment is not read. Environ replaces the process
environment, in KEY=value form, when it is not nil. Runtime supplies
settings stored outside the deployment and wins over every other layer.
*/
type LoadOptions struct {
	Name      string
	Files     []string
	EnvPrefix string
	Environ   []string
	Runtime   RuntimeStore
}

/*
RuntimeStore supplies settings changed while the application runs, such
as values an administrator saves in a settings screen.
*/
type RuntimeStore interface {
	/*
		Settings returns the stored values keyed by dotted path, such as
		"mail.default", or as nested maps.
	*/
	Settings(ctx context.Context) (map[string]any, error)
}

/*
Validator is implemented by configuration structs that check their own
values. Load calls Validate after decoding, so a bad value stops the
application at boot instead of on first use.
*/
type Validator interface {
	/*
		Validate reports the first value that cannot work.
	*/
	Validate() error
}

/*
Load builds a T from every layer. Fields start from their `default`
tags and are matched to configuration keys by their `koanf` tags. A
single string given for a list field, as every environment variable
is, is split on commas.
*/
func Load[T any](ctx context.Context, opts LoadOptions) (*T, error) {
	k, err := opts.merge(ctx)
	if err != nil {
		return nil, fmt.Errorf("load %s config: %w", opts.Name, err)
	}

	cfg := new(T)
	if err := applyDefaults(cfg); err != nil {
		return nil, fmt.Errorf("apply %s defaults: %w", opts.Name, err)
	}

	if err := splitLists(k, reflect.TypeFor[T]()); err != nil {
		return nil, fmt.Errorf("split %s lists: %w", opts.Name, err)
	}

	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, fmt.Errorf("decode %s config: %w", opts.Name, err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate %s config: %w", opts.Name, err)
	}

	return cfg, nil
}

func validate(cfg any) error {
	v, ok := cfg.(Validator)
	if !ok {
		return nil
	}

	return v.Validate()
}

func (o LoadOptions) merge(ctx context.Context) (*koanf.Koanf, error) {
	k := koanf.New(".")

	if err := o.loadFiles(k); err != nil {
		return nil, err
	}

	if err := o.loadEnv(k); err != nil {
		return nil, err
	}

	if err := o.loadRuntime(ctx, k); err != nil {
		return nil, err
	}

	return k, nil
}

/*
loadFiles reads the listed files, or the default Name.yaml when none are
listed. Only the default file may be missing, so an environment-only
deployment needs no file while a mistyped path still fails.
*/
func (o LoadOptions) loadFiles(k *koanf.Koanf) error {
	paths, optional := o.Files, false
	if len(paths) == 0 && o.Name != "" {
		paths, optional = []string{o.Name + ".yaml"}, true
	}

	for _, path := range paths {
		err := k.Load(file.Provider(path), yaml.Parser())
		if optional && errors.Is(err, fs.ErrNotExist) {
			continue
		}

		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
	}

	return nil
}

func (o LoadOptions) loadRuntime(ctx context.Context, k *koanf.Koanf) error {
	if o.Runtime == nil {
		return nil
	}

	settings, err := o.Runtime.Settings(ctx)
	if err != nil {
		return fmt.Errorf("read runtime settings: %w", err)
	}

	if err := k.Load(confmap.Provider(settings, "."), nil); err != nil {
		return fmt.Errorf("merge runtime settings: %w", err)
	}

	return nil
}
