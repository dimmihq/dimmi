/*
|--------------------------------------------------------------------------
| Written Schema Texts: Every Part
|--------------------------------------------------------------------------
|
| The parts the written schema texts are kept in, put together.
|
*/

package mcpserver

import "slices"

/*
schemaTextParts returns every part of the written schema texts.
*/
func schemaTextParts() []map[string]map[string]string {
	return slices.Concat(
		schemaTextPartsFromAIAction(),
		schemaTextPartsFromConversationRating(),
		schemaTextPartsFromMessageAIConfirmation(),
		schemaTextPartsFromSeriesTrace(),
		[]map[string]map[string]string{agentStudioSchemaTexts()},
	)
}

/*
schemaTextPartsFromAIAction returns the parts from AIAction on.
*/
func schemaTextPartsFromAIAction() []map[string]map[string]string {
	return []map[string]map[string]string{
		schemaTextsFromAIAction(),
		schemaTextsFromAIActionAndAIActionCall(),
		schemaTextsFromAIActionCall(),
		schemaTextsFromAIActionCallAndAIActionConnection(),
		schemaTextsFromAIActionConnection(),
		schemaTextsFromAIActionTest(),
		schemaTextsFromAIAgent(),
		schemaTextsFromAIAgentAndAIAgentView(),
		schemaTextsFromAIProviderKey(),
		schemaTextsFromAccessPolicy(),
		schemaTextsFromAiPlaygroundClass(),
		schemaTextsFromAiRun(),
		schemaTextsFromAiRunAndAiRunStep(),
		schemaTextsFromAiUsage(),
		schemaTextsFromAllowedAddressBody(),
		schemaTextsFromArticle(),
		schemaTextsFromAssignmentStatus(),
		schemaTextsFromAttribute(),
		schemaTextsFromAttributeProblem(),
		schemaTextsFromAudienceField(),
		schemaTextsFromAuditActor(),
		schemaTextsFromAuth(),
		schemaTextsFromAuthFactor(),
		schemaTextsFromBroadcast(),
		schemaTextsFromBroadcastAndBroadcastCompany(),
		schemaTextsFromBroadcastPreferences(),
		schemaTextsFromBroadcastRecipient(),
		schemaTextsFromBulkChangesOutputBody(),
		schemaTextsFromCatalogEntry(),
		schemaTextsFromChangelogEntry(),
		schemaTextsFromChangelogPerson(),
		schemaTextsFromCollection(),
		schemaTextsFromConfig(),
		schemaTextsFromConfigCompany(),
		schemaTextsFromConfigHomeLink(),
		schemaTextsFromConnectedApp(),
		schemaTextsFromContact(),
		schemaTextsFromContactIdentity(),
		schemaTextsFromContextCompany(),
		schemaTextsFromConversation(),
	}
}

/*
schemaTextPartsFromConversationRating returns the parts from
ConversationRating on.
*/
func schemaTextPartsFromConversationRating() []map[string]map[string]string {
	return []map[string]map[string]string{
		schemaTextsFromConversationRating(),
		schemaTextsFromConversationView(),
		schemaTextsFromCountViewsMentionsStruct(),
		schemaTextsFromCreatedIdentitySecret(),
		schemaTextsFromDNSSetupOffer(),
		schemaTextsFromDataStruct3On(),
		schemaTextsFromDomainRecord(),
		schemaTextsFromEmailAddress(),
		schemaTextsFromEmailAddressTest(),
		schemaTextsFromEmailDomain(),
		schemaTextsFromEmailSending(),
		schemaTextsFromEmailSignatureRendering(),
		schemaTextsFromEventDefinition(),
		schemaTextsFromFeedbackBoard(),
		schemaTextsFromFeedbackPost(),
		schemaTextsFromFeedbackStatus(),
		schemaTextsFromGroupRole(),
		schemaTextsFromHelpCenterDomain(),
		schemaTextsFromHelpCenterSettings(),
		schemaTextsFromHostGuide(),
		schemaTextsFromIdentitySecret(),
		schemaTextsFromIdentityValidation(),
		schemaTextsFromImport(),
		schemaTextsFromImportSummary(),
		schemaTextsFromInboundEmail(),
		schemaTextsFromInboxAssignment(),
		schemaTextsFromIntegration(),
		schemaTextsFromIntegrationFeature(),
		schemaTextsFromInvitation(),
		schemaTextsFromInvitationUser(),
		schemaTextsFromIssueSettings(),
		schemaTextsFromKnowledgeDocument(),
		schemaTextsFromKnowledgeSettings(),
		schemaTextsFromKnowledgeSource(),
		schemaTextsFromKnowledgeSourceType(),
		schemaTextsFromKnowledgeUpload(),
		schemaTextsFromLeafResult(),
		schemaTextsFromListedTag(),
		schemaTextsFromMember(),
		schemaTextsFromMembershipWorkspace(),
		schemaTextsFromMessageAI(),
	}
}

/*
schemaTextPartsFromMessageAIConfirmation returns the parts from
MessageAIConfirmation on.
*/
func schemaTextPartsFromMessageAIConfirmation() []map[string]map[string]string {
	return []map[string]map[string]string{
		schemaTextsFromMessageAIConfirmation(),
		schemaTextsFromMessengerAudiences(),
		schemaTextsFromMetaStruct7On(),
		schemaTextsFromNotification(),
		schemaTextsFromNotificationDelivery(),
		schemaTextsFromNotificationDestination(),
		schemaTextsFromNotificationSettings(),
		schemaTextsFromOAuthAuthorization(),
		schemaTextsFromOnboarding(),
		schemaTextsFromOperatorChangeset(),
		schemaTextsFromOperatorOperation(),
		schemaTextsFromOperatorRoutine(),
		schemaTextsFromOperatorRoutineAndOperatorRoutineCounts(),
		schemaTextsFromOperatorRoutineRun(),
		schemaTextsFromOperatorRoutineTemplate(),
		schemaTextsFromOperatorSuggestionOutcome(),
		schemaTextsFromOperatorUsage(),
		schemaTextsFromOperatorUsagePurpose(),
		schemaTextsFromOrganizationDomain(),
		schemaTextsFromPersonalAccessToken(),
		schemaTextsFromProcedureActionFigure(),
		schemaTextsFromProcedureMetrics(),
		schemaTextsFromProcedureRunFigure(),
		schemaTextsFromProcedureTestCase(),
		schemaTextsFromProvider(),
		schemaTextsFromRatingFace(),
		schemaTextsFromReport(),
		schemaTextsFromReportConversation(),
		schemaTextsFromReportMetric(),
		schemaTextsFromReportRow(),
		schemaTextsFromRetention(),
		schemaTextsFromSLAClock(),
		schemaTextsFromSLAPolicy(),
		schemaTextsFromSSOConnection(),
		schemaTextsFromSSOLookup(),
		schemaTextsFromSchedule(),
		schemaTextsFromSecurityEvent(),
		schemaTextsFromSeries(),
		schemaTextsFromSeriesEnrollment(),
		schemaTextsFromSeriesStep(),
	}
}

/*
schemaTextPartsFromSeriesTrace returns the parts from SeriesTrace on.
*/
func schemaTextPartsFromSeriesTrace() []map[string]map[string]string {
	return []map[string]map[string]string{
		schemaTextsFromSeriesTrace(),
		schemaTextsFromSeriesVersion(),
		schemaTextsFromSettings(),
		schemaTextsFromSimHandoff(),
		schemaTextsFromSkippedTool(),
		schemaTextsFromSlackRoom(),
		schemaTextsFromSnoozeLists(),
		schemaTextsFromSocialChannel(),
		schemaTextsFromSocialProfile(),
		schemaTextsFromTagCounts(),
		schemaTextsFromTallyBroadcastRecipientsRow(),
		schemaTextsFromTestFinding(),
		schemaTextsFromTicket(),
		schemaTextsFromTicketField(),
		schemaTextsFromTicketFieldReport(),
		schemaTextsFromTicketIssue(),
		schemaTextsFromTicketSettings(),
		schemaTextsFromTicketSummary(),
		schemaTextsFromTicketType(),
		schemaTextsFromTimelineEntry(),
		schemaTextsFromTimelineEntryAndTokenAbility(),
		schemaTextsFromTrackedEvent(),
		schemaTextsFromTrackerConversation(),
		schemaTextsFromUser(),
		schemaTextsFromViewWarning(),
		schemaTextsFromWebhookDelivery(),
		schemaTextsFromWebhookDeliveryDetail(),
		schemaTextsFromWebhookEndpoint(),
		schemaTextsFromWebhookSecret(),
		schemaTextsFromWhatsAppChannel(),
		schemaTextsFromWhatsAppHookup(),
		schemaTextsFromWhatsAppTemplate(),
		schemaTextsFromWidgetPreviewConfig(),
		schemaTextsFromWidgetSettings(),
		schemaTextsFromWorkflow(),
		schemaTextsFromAutomation(),
		schemaTextsFromAutomationFailures(),
		schemaTextsFromWorkflowRunStep(),
		schemaTextsFromWorkspace(),
		schemaTextsFromWorkspaceMember(),
	}
}
