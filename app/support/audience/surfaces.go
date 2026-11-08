/*
|--------------------------------------------------------------------------
| Surfaces
|--------------------------------------------------------------------------
|
| Each place a rule guards is registered once with its gate and where
| its rules run. Workflows route rather than reveal, so both of their
| surfaces are display surfaces evaluated on a conversation, where
| contact, company and conversation facts are all at hand.
|
*/

package audience

/*
WorkflowAudience is the surface of a workflow's audience: who a
workflow runs for.
*/
func WorkflowAudience() Surface {
	return Surface{Key: "workflow.audience", Gate: GateDisplay, Where: []Location{Server, Conversation}}
}

/*
WorkflowBranch is the surface of a workflow's branch conditions.
*/
func WorkflowBranch() Surface {
	return Surface{Key: "workflow.branch", Gate: GateDisplay, Where: []Location{Server, Conversation}}
}

/*
WorkflowOpener is the surface of an opener workflow's audience, which
the widget decides before any conversation exists: page fields are
known in the browser, conversation fields are not known at all.
*/
func WorkflowOpener() Surface {
	return Surface{Key: "workflow.opener", Gate: GateDisplay, Where: []Location{Server, Client}}
}

/*
SavedAudience is the surface of a saved audience. It may be used
anywhere, so it reads the person and the page they are on, never a
conversation, which only some surfaces have.
*/
func SavedAudience() Surface {
	return Surface{Key: "audience.saved", Gate: GateDisplay, Where: []Location{Server, Client}}
}
