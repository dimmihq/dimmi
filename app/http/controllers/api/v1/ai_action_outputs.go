/*
|--------------------------------------------------------------------------
| Agent Action Outputs
|--------------------------------------------------------------------------
|
| The typed response bodies of an action's versions and tests.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
AIActionVersionOutput is one version.
*/
type AIActionVersionOutput struct {
	Body fhttp.Resource[resources.AIActionVersion]
}

/*
AIActionVersionsOutput is an action's versions.
*/
type AIActionVersionsOutput struct {
	Body fhttp.Collection[resources.AIActionVersion]
}

/*
AIActionTestOutput is a test's result.
*/
type AIActionTestOutput struct {
	Body fhttp.Resource[resources.AIActionTest]
}
