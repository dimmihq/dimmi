/*
|--------------------------------------------------------------------------
| Paths Of A Step
|--------------------------------------------------------------------------
|
| Which paths each step leaves by, and which kinds of step exist, for
| callers that build or change a series outside the builder, such as
| Operator. The checker reads the same lists.
|
*/

package series

/*
StepPaths lists the paths a step can leave by: next for most steps, the
fixed paths of its kind, and the paths its settings name.
*/
func StepPaths(n Node) []string {
	return pathsOf(n)
}

/*
IsStepKind reports whether a kind of step exists.
*/
func IsStepKind(kind string) bool {
	return knownKind(kind)
}
