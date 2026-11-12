/*
|--------------------------------------------------------------------------
| Engineering Trackers
|--------------------------------------------------------------------------
|
| The tools engineering works in, seen from support: issue trackers
| (Linear, Jira, GitHub Issues) and incident tools (incident.io, Rootly,
| PagerDuty). Each speaks its own API, and each is wrapped in the same
| shape: lists to pick from (teams, projects, states, severities,
| services), search, read, create, comment, and the impact on customers
| written back so engineering ranks work by it. Every status a tool
| reports falls into one of five groups, so dimmi maps them to its own
| states the same way whatever the tool.
|
| Deliveries from each tool are checked with its own signature and read
| into the same events: an issue or incident changed, a comment, a new
| incident, a deployment.
|
*/

package trackers

import (
	"context"
	"errors"
	"time"
)

/*
The status groups every tool's statuses fall into.
*/
const (
	GroupBacklog   = "backlog"
	GroupUnstarted = "unstarted"
	GroupStarted   = "started"
	GroupDone      = "done"
	GroupCanceled  = "canceled"
)

/*
The lists a tracker offers to pick from.
*/
const (
	ListTeams        = "teams"
	ListProjects     = "projects"
	ListStates       = "states"
	ListSeverities   = "severities"
	ListServices     = "services"
	ListUsers        = "users"
	ListEnvironments = "environments"
)

/*
The kinds of event a delivery carries.
*/
const (
	EventIssue      = "issue"
	EventComment    = "comment"
	EventIncident   = "incident_created"
	EventUpdate     = "public_update"
	EventDeployment = "deployment"
)

/*
Failures of a tracker call.
*/
var (
	ErrNotFound     = errors.New("trackers: no such issue")
	ErrAuth         = errors.New("trackers: the tool refused the token")
	ErrHidden       = errors.New("trackers: the app cannot see this team")
	ErrUnsupported  = errors.New("trackers: the tool does not offer this")
	ErrBadSignature = errors.New("trackers: the delivery's signature does not match")
	ErrProvider     = errors.New("trackers: the tool answered with an error")
)

/*
Call is what a call needs: the installation's token and the facts kept
in its metadata, such as a Jira site's cloud id.
*/
type Call struct {
	Token string
	Meta  map[string]any
}

/*
Meta reads a string the installation's metadata holds.
*/
func (c Call) meta(key string) string {
	s, _ := c.Meta[key].(string)

	return s
}

/*
Option is one value to pick: its id and name, a detail line, the status
group of a workflow state and the rank of a severity, higher the worse.
*/
type Option struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	Group  string `json:"group,omitempty"`
	Rank   int    `json:"rank,omitempty"`
}

/*
Issue is an issue or incident as the tool reports it.
*/
type Issue struct {
	ExternalID  string
	Key         string
	URL         string
	Title       string
	Status      string
	StatusID    string
	StatusGroup string
	TeamID      string
	TeamName    string
	Severity    string
	SeverityID  string
	Rank        int
}

/*
Impact is what an issue costs customers: how many customers,
companies and conversations it reaches, their monthly revenue, the
largest companies by name and the link back to the tracker.
*/
type Impact struct {
	Customers     int64
	Companies     int64
	Conversations int64
	Revenue       float64
	CompanyNames  []string
	Link          string
}

/*
CreateInput is a new issue or incident: where it goes (team, project,
service), its title and description, the severity of an incident and
the impact to write on it.
*/
type CreateInput struct {
	Team        string
	Project     string
	Service     string
	Severity    string
	Title       string
	Description string
	Impact      Impact
}

/*
Tracker is one engineering tool.
*/
type Tracker interface {
	/*
		Provider returns the tool's registry name.
	*/
	Provider() string

	/*
		Incidents reports whether the tool holds incidents, not issues.
	*/
	Incidents() bool

	/*
		Options returns a list to pick from, within the parent named.
	*/
	Options(ctx context.Context, c Call, list, parent string) ([]Option, error)

	/*
		Search finds issues by words or a key.
	*/
	Search(ctx context.Context, c Call, q string) ([]Issue, error)

	/*
		Read returns the issue an id, key or URL names.
	*/
	Read(ctx context.Context, c Call, ref string) (Issue, error)

	/*
		Create files a new issue or incident.
	*/
	Create(ctx context.Context, c Call, in CreateInput) (Issue, error)

	/*
		Comment posts a comment or note on the issue.
	*/
	Comment(ctx context.Context, c Call, is Issue, body string) error

	/*
		Impact writes the impact on customers to the issue.
	*/
	Impact(ctx context.Context, c Call, is Issue, impact Impact) error

	/*
		Verify checks a delivery was signed with the secret.
	*/
	Verify(s Signed, secret string, now time.Time) error

	/*
		Parse reads a verified delivery's events.
	*/
	Parse(body []byte) ([]Event, error)
}

/*
Set holds the trackers by provider name.
*/
type Set map[string]Tracker

/*
Get returns the provider's tracker.
*/
func (s Set) Get(provider string) (Tracker, bool) {
	t, ok := s[provider]

	return t, ok
}
