/*
|--------------------------------------------------------------------------
| Portal Status
|--------------------------------------------------------------------------
|
| The help center's status page, public to everyone, laid out the way
| customers know status pages: one line for the whole product, each
| affected area with its current status, the incidents active now with
| their public log, then those resolved in the last ninety days.
|
*/

package helpcenter

import "strings"

/*
Incident is one incident on the status page: its title, the label
customers read for its state, its status tone, the areas it touches,
when it started and resolved, and its public log, newest first.
*/
type Incident struct {
	Title    string
	Label    string
	Status   string
	Areas    []string
	Started  string
	Resolved string
	Updates  []IncidentPost
}

/*
IncidentPost is one post of an incident's public log.
*/
type IncidentPost struct {
	Body string
	At   string
}

/*
Component is one affected area and its status: operational, degraded,
partial_outage or major_outage.
*/
type Component struct {
	Name   string
	Status string
}

/*
SystemStatus is the status page.
*/
type SystemStatus struct {
	Overall    string
	Components []Component
	Active     []Incident
	History    []Incident
}

/*
healthText is what a status reads as.
*/
func healthText(status string) string {
	switch status {
	case "degraded":
		return "Degraded performance"
	case "partial_outage":
		return "Partial outage"
	case "major_outage":
		return "Major outage"
	}

	return "Operational"
}

/*
healthOverall is the page's one line.
*/
func healthOverall(status string) string {
	if status == "" || status == "operational" {
		return "All systems operational"
	}

	return healthText(status)
}

/*
healthTone colors a status.
*/
func healthTone(status string) string {
	switch status {
	case "degraded":
		return "bg-status-yellow-background text-status-yellow-text"
	case "partial_outage":
		return "bg-status-orange-background text-status-orange-text"
	case "major_outage":
		return "bg-status-rose-background text-status-rose-text"
	}

	return "bg-status-lime-background text-status-lime-text"
}

/*
healthIcon is the icon beside the page's one line.
*/
func healthIcon(status string) string {
	if status == "" || status == "operational" {
		return "check"
	}

	return "alarm-warning"
}

/*
joinAreas lists an incident's areas.
*/
func joinAreas(areas []string) string {
	return strings.Join(areas, ", ")
}
