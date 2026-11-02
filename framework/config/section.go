/*
|--------------------------------------------------------------------------
| Configuration Sections
|--------------------------------------------------------------------------
|
| A section is one raw branch of the configuration, such as a single disk
| under filesystems.disks. Managers keep sections raw until they know which
| driver an entry names, because each driver decodes its own options and
| an S3 disk and a local disk share almost none.
|
*/

package config

import (
	"fmt"
	"reflect"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
)

/*
Section is a raw configuration branch, keyed by option name.
*/
type Section map[string]any

/*
String returns the named option when it is a string, else "".
*/
func (s Section) String(key string) string {
	v, _ := s[key].(string)

	return v
}

/*
Decode fills the target from the section. Each field starts from its
`default` tag, then takes the section's value for its `koanf` tag. A
single string given for a list field is split on commas.
*/
func (s Section) Decode(target any) error {
	if err := applyDefaults(target); err != nil {
		return fmt.Errorf("apply defaults: %w", err)
	}

	k := koanf.New(".")
	if err := k.Load(confmap.Provider(s, ""), nil); err != nil {
		return fmt.Errorf("load section: %w", err)
	}

	if err := splitLists(k, reflect.TypeOf(target)); err != nil {
		return fmt.Errorf("split section lists: %w", err)
	}

	if err := k.UnmarshalWithConf("", target, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return fmt.Errorf("decode section: %w", err)
	}

	return nil
}
