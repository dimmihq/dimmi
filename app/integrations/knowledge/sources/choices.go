/*
|--------------------------------------------------------------------------
| Choosing Settings
|--------------------------------------------------------------------------
|
| A setting that names something at the service, such as a repository
| or a branch, is picked from what the service holds rather than typed.
| Its schema field has the format "x-choice", and "x-depends" lists the
| settings it is read with, so the form asks again when one of them
| changes. The driver answers with every option and marks the one to
| preselect, such as a repository's default branch.
|
*/

package sources

import "context"

/*
Option is one value a setting can take: the value stored, the name
shown, a short detail beside it, and whether it is preselected.
*/
type Option struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Detail  string `json:"detail,omitempty"`
	Default bool   `json:"default,omitempty"`
}

/*
Chooser is a driver that lists the values of its x-choice settings.
*/
type Chooser interface {
	/*
		Options lists the values the named setting can take, read with
		the settings filled so far.
	*/
	Options(ctx context.Context, cfg Config, field string) ([]Option, error)
}
