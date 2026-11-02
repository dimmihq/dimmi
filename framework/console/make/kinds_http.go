/*
|--------------------------------------------------------------------------
| HTTP Kinds
|--------------------------------------------------------------------------
|
| The request lifecycle has a file for each step: a controller per
| resource under app/http/controllers, grouped by audience and version,
| the request struct it binds and validates in app/http/requests, the
| policy that decides who may act on a resource in app/policies, and the
| notifications sent about it in app/notifications.
|
*/

package make

import (
	"fmt"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/support/str"
)

func httpKinds() []kind {
	return []kind{
		{
			name: "controller", description: "Create a controller", holds: "HTTP controllers",
			argument: "the controller, in its folder: api/v1/TagController",
			plan:     nested("app/http/controllers", "Controller", "_controller.go", "controller.go.tmpl"),
		},
		{
			name: "request", description: "Create a form request", holds: "HTTP request inputs",
			argument: "the request: StoreTagRequest", flags: requestFlags, plan: planRequest,
		},
		{
			name: "policy", description: "Create an authorization policy", holds: "authorization policies",
			argument: "the policy: TagPolicy", flags: policyFlags, plan: planPolicy,
		},
		{
			name: "notification", description: "Create a notification",
			holds:    "the notifications the application sends",
			argument: "the notification: InvoicePaidNotification",
			plan:     suffixed("app/notifications", "Notification", "notification.go.tmpl"),
		},
	}
}

func requestFlags(flags *console.Flags) {
	flags.Bool("authorize", "add an Authorize hook that decides whether the caller may make the request")
	flags.Bool("validate", "add a Validate hook for rules the tags cannot express")
}

func policyFlags(flags *console.Flags) {
	flags.String("model", "", "the model type it decides about, from app/models")
}

/*
suffixed plans a flat file named after the type with the suffix removed
and the snake case suffix added, so "StoreTag" and "StoreTagRequest"
both give store_tag_request.go.
*/
func suffixed(dir, suffix, stub string) func(name, *console.Input, Generator) (file, error) {
	return func(n name, in *console.Input, g Generator) (file, error) {
		return flat(dir, "_"+str.Snake(suffix)+".go", stub)(n.without(suffix), in, g)
	}
}

func planRequest(n name, in *console.Input, g Generator) (file, error) {
	f, err := suffixed("app/http/requests", "Request", "request.go.tmpl")(n, in, g)
	if err != nil {
		return file{}, err
	}

	f.data.Authorize = in.Bool("authorize")
	f.data.Validate = in.Bool("validate")

	return f, nil
}

/*
planPolicy names the policy's permissions after the plural of what it
decides about, "tags.update" for TagPolicy, and types the resource as
the model when one is given.
*/
func planPolicy(n name, in *console.Input, g Generator) (file, error) {
	f, err := suffixed("app/policies", "Policy", "policy.go.tmpl")(n, in, g)
	if err != nil {
		return file{}, err
	}

	f.data.Permission = str.Kebab(str.Plural(f.data.Name))
	f.data.Resource = "any"

	if model := in.String("model"); model != "" {
		if !typeRe.MatchString(model) {
			return file{}, fmt.Errorf("%w: model %q", ErrInvalidName, model)
		}

		f.data.Model = model
		f.data.Resource = "*models." + model
	}

	return f, nil
}
