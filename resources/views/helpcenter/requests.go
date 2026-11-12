/*
|--------------------------------------------------------------------------
| Portal Requests
|--------------------------------------------------------------------------
|
| A signed-in reader's requests to the team: the list of theirs, and of
| their company's when a type shares them, the form of each type, and a
| request's own page with where it stands, its thread and a reply box.
| States show by the label customers read, never the team's own name.
|
*/

package helpcenter

/*
Request is one request in a list, or the head of its page.
*/
type Request struct {
	Ref      string
	Title    string
	Type     string
	TypeIcon string
	Label    string
	Group    string
	Updated  string
	Mine     bool
}

/*
RequestType is a type of request a reader may open, by public id.
*/
type RequestType struct {
	ID          string
	Name        string
	Icon        string
	Description string
}

/*
RequestField is a field of a request's form or page. ShowField and
ShowEquals say the field shows only once another has a value, one of
ShowEquals when it lists any.
*/
type RequestField struct {
	Key         string
	Label       string
	Description string
	DataType    string
	Options     []string
	Required    bool
	ShowField   string
	ShowEquals  string
	Value       string
}

/*
Requests is the reader's list: theirs or their company's, with the
types they may open and whether the company tab shows.
*/
type Requests struct {
	Scope   string
	Company bool
	Items   []Request
	Types   []RequestType
}

/*
RequestForm is the form of one type with what the reader typed, and
why it was refused when it was.
*/
type RequestForm struct {
	Type        RequestType
	Types       []RequestType
	Fields      []RequestField
	Title       string
	Description string
	Error       string
	Key         string
}
