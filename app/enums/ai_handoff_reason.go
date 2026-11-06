/*
|--------------------------------------------------------------------------
| AI Handoff Reason
|--------------------------------------------------------------------------
|
| Every handoff names why the agent stepped aside, so the team can
| see the pattern: customers asking, the agent not knowing, rules
| firing, or safety.
|
*/

package enums

/*
AIHandoffReason names why the agent stopped answering a conversation.
*/
type AIHandoffReason string

/*
The handoff reasons.
*/
const (
	AIHandoffCustomerRequested AIHandoffReason = "customer_requested"
	AIHandoffUnknown           AIHandoffReason = "unknown"
	AIHandoffUnknownStreak     AIHandoffReason = "unknown_streak"
	AIHandoffMaxReplies        AIHandoffReason = "max_replies"
	AIHandoffFrustration       AIHandoffReason = "frustration"
	AIHandoffFeedback          AIHandoffReason = "feedback"
	AIHandoffAbusive           AIHandoffReason = "abusive"
	AIHandoffSafety            AIHandoffReason = "safety"
	AIHandoffUnsupportedLang   AIHandoffReason = "unsupported_language"
	AIHandoffProviderError     AIHandoffReason = "provider_error"
	AIHandoffSpendCap          AIHandoffReason = "spend_cap"
	AIHandoffBillingPaused     AIHandoffReason = "billing_paused"
	AIHandoffSchedule          AIHandoffReason = "schedule"
	AIHandoffHumanReplied      AIHandoffReason = "human_replied"
	AIHandoffMemberPaused      AIHandoffReason = "member_paused"
	AIHandoffAgentDisabled     AIHandoffReason = "agent_disabled"
)
