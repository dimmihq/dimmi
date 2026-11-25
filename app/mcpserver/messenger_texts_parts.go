/*
|--------------------------------------------------------------------------
| The Messenger API's Words: Every Part
|--------------------------------------------------------------------------
|
| The parts the messenger API's texts are kept in, put together.
|
*/

package mcpserver

/*
messengerTextParts returns every part of the messenger API's texts.
*/
func messengerTextParts() []map[string]map[string]string {
	return []map[string]map[string]string{
		messengerTextsFromAvailabilityResource(),
		messengerTextsFromCheckWidgetEmailCodeRequestBody(),
		messengerTextsFromEmailCodeSent(),
		messengerTextsFromHelpIndex(),
		messengerTextsFromRealtimeTokenResource(),
		messengerTextsFromStoreWidgetHelpPassRequestBody(),
		messengerTextsFromStoreWidgetTicketRequestBody(),
		messengerTextsFromUpdateWidgetActionCallRequestBody(),
		messengerTextsFromWidgetActionCall(),
		messengerTextsFromWidgetChoice(),
		messengerTextsFromWidgetIdentity(),
		messengerTextsFromWidgetMessageBody(),
		messengerTextsFromWidgetSession(),
		messengerTextsFromWidgetTicketBlock(),
		messengerTextsFromWidgetTicketType(),
	}
}
