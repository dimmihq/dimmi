/*
|--------------------------------------------------------------------------
| Route Lint
|--------------------------------------------------------------------------
|
| Anyone who knows one resource should be able to guess every other, so
| the API's routes follow a few strict rules: paths are nouns, PUT only
| replaces a singleton's whole content, and a route's name follows its
| path the way resource routes are named. The rules are checked here, in
| code, so a verb in a path fails the build instead of a review. Which
| words count as verbs, where PUT is allowed and which paths are exempt
| is the application's choice.
|
*/

package routing

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/http"
)

/*
Rules configure the route lint.

VerbDenylist lists words that may not be a path segment, such as "close"
or "login". PutAllowlist lists the paths that may use PUT. Prefixes
limits the lint to routes under them; with none, every route is checked.
Exceptions lists paths the lint skips; one ending in "*" skips every
path that starts with the rest.
*/
type Rules struct {
	VerbDenylist []string
	PutAllowlist []string
	Prefixes     []string
	Exceptions   []string
}

/*
Lint checks the routes against the rules and returns one line per
problem, naming the route; none means the routes pass.
*/
func Lint(routes []*Route, rules Rules) []string {
	var problems []string

	for _, route := range routes {
		prefix, ok := rules.covers(route.path)
		if !ok {
			continue
		}

		for _, problem := range lintRoute(route, prefix, rules) {
			problems = append(problems, fmt.Sprintf("%s %s: %s", route.method, route.path, problem))
		}
	}

	return problems
}

/*
covers reports whether the rules apply to the path and returns the
prefix it sits under.
*/
func (rules Rules) covers(path string) (string, bool) {
	if http.MatchesPath(path, rules.Exceptions...) {
		return "", false
	}

	if len(rules.Prefixes) == 0 {
		return "", true
	}

	for _, prefix := range rules.Prefixes {
		if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
			return strings.TrimSuffix(prefix, "/"), true
		}
	}

	return "", false
}

func lintRoute(route *Route, prefix string, rules Rules) []string {
	var problems []string

	nouns, end := shape(strings.TrimPrefix(route.path, prefix))
	for _, noun := range nouns {
		if slices.Contains(rules.VerbDenylist, noun) {
			problems = append(problems, fmt.Sprintf("path segment %q is a verb; use a noun", noun))
		}
	}

	if route.method == http.MethodPut && !slices.Contains(rules.PutAllowlist, route.path) {
		problems = append(problems, "PUT only replaces a singleton's whole content; use PATCH")
	}

	if problem := lintName(route, nouns, end); problem != "" {
		problems = append(problems, problem)
	}

	return problems
}

/*
lintName checks that the name is the path's nouns joined by dots,
followed by the resource action the method and path shape allow.
*/
func lintName(route *Route, nouns []string, end string) string {
	if route.name == "" {
		return "route has no name"
	}

	actions := resourceActions()[route.method+" "+end]
	if len(actions) == 0 {
		return fmt.Sprintf("%s is not a resource action on this path", route.method)
	}

	base := strings.Join(nouns, ".")
	names := make([]string, len(actions))
	for i, action := range actions {
		names[i] = strings.TrimPrefix(base+"."+action, ".")
	}

	if slices.Contains(names, route.name) {
		return ""
	}

	return fmt.Sprintf("name %q should be one of %s", route.name, strings.Join(names, ", "))
}

/*
shape returns the fixed segments of a path and what it ends at: a
member, when the last segment is a parameter, or a collection, which
includes singletons.
*/
func shape(path string) ([]string, string) {
	var nouns []string

	end := collection
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "" {
			continue
		}

		if _, ok := placeholderName(segment); ok {
			end = member

			continue
		}

		nouns = append(nouns, segment)
		end = collection
	}

	return nouns, end
}

/*
The ends a path can have, as shape reports them.
*/
const (
	member     = "member"
	collection = "collection"
)

/*
resourceActions lists the actions each method performs, keyed by the
method and the end of the path it is sent to.
*/
func resourceActions() map[string][]string {
	return map[string][]string{
		http.MethodGet + " " + member:        {"show"},
		http.MethodPatch + " " + member:      {"update"},
		http.MethodPut + " " + member:        {"update"},
		http.MethodDelete + " " + member:     {"destroy"},
		http.MethodGet + " " + collection:    {"index", "show"},
		http.MethodPost + " " + collection:   {"store"},
		http.MethodPatch + " " + collection:  {"update", "update-many"},
		http.MethodPut + " " + collection:    {"update"},
		http.MethodDelete + " " + collection: {"destroy", "destroy-many"},
	}
}
