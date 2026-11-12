/*
|--------------------------------------------------------------------------
| GitHub Deliveries
|--------------------------------------------------------------------------
|
| A repository's webhook delivers issue changes, new comments and
| deployment statuses, each told apart by its shape: a deployment
| status names its environment and whether it succeeded, a comment
| carries its issue, and an issue change the issue itself. Comments on
| pull requests are left out.
|
*/

package trackers

/*
githubDelivery is a GitHub webhook payload.
*/
type githubDelivery struct {
	Action     string       `json:"action"`
	Issue      *githubIssue `json:"issue"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Comment *struct {
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Status *struct {
		State       string `json:"state"`
		Environment string `json:"environment"`
	} `json:"deployment_status"`
}

/*
Parse reads a GitHub delivery.
*/
func (*GitHub) Parse(body []byte) ([]Event, error) {
	var d githubDelivery
	if err := decode(body, &d); err != nil {
		return nil, err
	}

	repo := d.Repository.FullName

	switch {
	case d.Status != nil:
		return []Event{{
			Kind: EventDeployment, Repository: repo, Environment: d.Status.Environment,
			Succeeded: d.Status.State == "success",
		}}, nil
	case d.Issue == nil || d.Issue.PullRequest != nil:
		return nil, nil
	case d.Comment != nil:
		if d.Action != "created" || stringsHasMarker(d.Comment.Body) {
			return nil, nil
		}

		is := d.Issue.issue(repo)

		return []Event{{
			Kind: EventComment, ExternalID: is.ExternalID, Body: d.Comment.Body, Author: d.Comment.User.Login,
		}}, nil
	default:
		is := d.Issue.issue(repo)

		return []Event{{Kind: EventIssue, ExternalID: is.ExternalID, Issue: &is}}, nil
	}
}

/*
stringsHasMarker reports dimmi's own impact comment, which never comes
back as a note.
*/
func stringsHasMarker(body string) bool {
	return len(body) >= len(impactMarker) && body[:len(impactMarker)] == impactMarker
}
