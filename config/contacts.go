/*
|--------------------------------------------------------------------------
| Contacts Settings
|--------------------------------------------------------------------------
|
| How long a customer someone asked to erase stays recoverable before the
| wipe, so a request made by mistake can be withdrawn. Zero wipes as soon
| as the job runs.
|
*/

package config

/*
ContactsConfig configures customer records. EraseGraceDays is how many
days an erasure waits before wiping, and ExportTTLHours how long a
customer's data export can be downloaded.
*/
type ContactsConfig struct {
	EraseGraceDays int `default:"7"  koanf:"erase_grace_days"`
	ExportTTLHours int `default:"24" koanf:"export_ttl_hours"`
}
