/*
|--------------------------------------------------------------------------
| What A Step Still Needs
|--------------------------------------------------------------------------
|
| When dimmi reports a procedure step as done, it also lists what the
| step asked the customer for that they have not given. A step with
| anything on that list is not finished. It is a list on purpose:
| "nothing is missing" has to be something the code can read without
| interpreting a sentence, and an empty list is that.
|
| A model sometimes sends a sentence where the list belongs, such as
| "Nothing." Whether such a sentence names something missing is not
| for code to guess. The report is sent back with a note asking for
| the list, and dimmi reports again.
|
*/

package pipeline

import (
	"encoding/json"
	"strings"
)

/*
missing is what a step still needs from the customer, as dimmi lists
it: one item each, and none when the step is covered. Sentence is set
when a sentence came where the list belongs.
*/
type missing struct {
	Items    []string
	Sentence bool
}

/*
UnmarshalJSON reads the list. An empty text is read as an empty list;
any other text is kept as a sentence to send back.
*/
func (m *missing) UnmarshalJSON(raw []byte) error {
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		*m = missing{Sentence: strings.TrimSpace(text) != ""}
		return nil
	}
	*m = missing{}
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			m.Items = append(m.Items, item)
		}
	}
	return nil
}

/*
listNote sends a step report back when a sentence came where the list
of what is missing belongs.
*/
func listNote() (string, error) {
	out, err := json.Marshal(map[string]any{
		"ok": false,
		"note": "still_missing must be a list, not a sentence. Send [] when the customer gave everything the step " +
			"asks for or the step asks them for nothing. Otherwise send one item for each thing still needed. " +
			"Report the step again.",
	})
	return string(out), err
}
