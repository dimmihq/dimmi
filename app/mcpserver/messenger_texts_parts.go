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
		messengerTexts1(), messengerTexts2(), messengerTexts3(), messengerTexts4(), messengerTexts5(),
		messengerTexts6(), messengerTexts7(), messengerTexts8(), messengerTexts9(), messengerTexts10(),
		messengerTexts11(), messengerTexts12(), messengerTexts13(), messengerTexts14(),
		messengerTexts15(),
	}
}
