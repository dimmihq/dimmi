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

/*
MessengerDisplay is the surface of what the messenger shows: its
launcher and its tabs. Showing reveals nothing, so a browser's claims may
decide it, and page conditions are checked in the browser on each page.
*/
func MessengerDisplay(key string) Surface {
	return Surface{Key: "messenger." + key, Gate: GateDisplay, Where: []Location{Server, Client}}
}

/*
MessengerStart is the surface of who may start a conversation. It
spends the team's time, so only what the server knows and trusts
decides it: no page conditions, no value a browser could claim.
*/
func MessengerStart() Surface {
	return Surface{Key: "messenger.start", Gate: GateAccess, Where: []Location{Server}}
}
