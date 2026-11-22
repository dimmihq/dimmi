/*
|--------------------------------------------------------------------------
| Procedure Test Case Requests
|--------------------------------------------------------------------------
|
| One saved test named in the path, its changes, and the tests a run
| takes.
|
*/

package requests

/*
TestCaseRequest names a test case of a procedure in the path.
*/
type TestCaseRequest struct {
	Procedure string `path:"procedure"`
	TestCase  string `path:"test_case"`
}

/*
UpdateTestCaseRequest is the input of PATCH
/procedures/{procedure}/test-cases/{test_case}.
*/
type UpdateTestCaseRequest struct {
	Procedure string `path:"procedure"`
	TestCase  string `path:"test_case"`
	Body      TestCaseBody
}

/*
RunTestCasesRequest is the input of POST
/procedures/{procedure}/test-cases/run: the tests to run, every one when
none is named. StreamID, chosen by the caller, makes the run report each
turn as it happens on the caller's own realtime channel.
*/
type RunTestCasesRequest struct {
	Procedure string `path:"procedure"`
	Body      *struct {
		TestCaseIDs []string `json:"test_case_ids,omitempty" maxItems:"20"`
		StreamID    string   `json:"stream_id,omitempty"     maxLength:"64"`
	}
}
