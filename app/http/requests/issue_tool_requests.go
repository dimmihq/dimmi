/*
|--------------------------------------------------------------------------
| Issue Tool Requests
|--------------------------------------------------------------------------
|
| The inputs of a connected tool's lists, search, sync settings and of connecting a tool
| with an API key.
|
*/

package requests

/*
IssueSettingsRequest names an installation's sync settings.
*/
type IssueSettingsRequest struct {
	Installation string `path:"installation"`
}

/*
UpdateIssueSettingsRequest changes a tool's sync settings and, for a
tool connected with a key, the secret its deliveries are signed with.
States maps the tool's status ids to dimmi state ids.
*/
type UpdateIssueSettingsRequest struct {
	Installation string `path:"installation"`
	Body         struct {
		States            map[string]string `json:"states,omitempty"`
		TeamID            *string           `json:"team_id,omitempty"            maxLength:"200"`
		ProjectID         *string           `json:"project_id,omitempty"         maxLength:"200"`
		ServiceID         *string           `json:"service_id,omitempty"         maxLength:"200"`
		SeverityID        *string           `json:"severity_id,omitempty"        maxLength:"200"`
		MirrorSeverityID  *string           `json:"mirror_severity_id,omitempty" maxLength:"200"`
		FromEmail         *string           `json:"from_email,omitempty"         maxLength:"320"`
		ImpactFieldID     *string           `json:"impact_field_id,omitempty"    maxLength:"200"`
		DeployRepository  *string           `json:"deploy_repository,omitempty"  maxLength:"200"`
		DeployEnvironment *string           `json:"deploy_environment,omitempty" maxLength:"200"`
		SigningSecret     string            `json:"signing_secret,omitempty"     maxLength:"500"`
	}
}

/*
StoreKeyInstallationRequest connects a tool with an API key.
*/
type StoreKeyInstallationRequest struct {
	Provider string `path:"provider"`
	Body     struct {
		APIKey        string `json:"api_key"                  maxLength:"500" validate:"required"`
		SigningSecret string `json:"signing_secret,omitempty" maxLength:"500"`
	}
}
