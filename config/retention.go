/*
|--------------------------------------------------------------------------
| Retention
|--------------------------------------------------------------------------
|
| How long the logs people answer for are kept. A workspace's audit log
| is kept a year on the cloud and for ever on a self-hosted install
| unless its operator sets a limit. A person's own security log is kept
| a year on both, long enough to look back on a suspicious sign-in.
|
*/

package config

/*
DefaultRetentionDays is how long the cloud keeps audit entries, and
both editions keep security log entries, unless told otherwise.
*/
const DefaultRetentionDays = 365

/*
AuditConfig says how long audit entries are kept. RetentionDays is the
number of days, 0 to keep them for ever; unset, it is a year on the
cloud and for ever on a self-hosted install.
*/
type AuditConfig struct {
	RetentionDays *int `koanf:"retention_days"`
}

/*
SecurityEventsConfig says how long security log entries are kept, in
days, 0 to keep them for ever.
*/
type SecurityEventsConfig struct {
	RetentionDays int `default:"365" koanf:"retention_days"`
}

/*
settle fills the audit retention the edition implies.
*/
func (c *AuditConfig) settle(edition Edition) {
	if c.RetentionDays != nil {
		return
	}

	days := 0
	if edition == EditionCloud {
		days = DefaultRetentionDays
	}

	c.RetentionDays = &days
}
