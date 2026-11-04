//go:build emailmatrix

/*
|--------------------------------------------------------------------------
| Harness Settings
|--------------------------------------------------------------------------
|
| A run sends real mail and spends real money, so it starts only when every
| credential it needs is present. They come from the environment or from one
| Secrets Manager document, read once at start, never from a file in the
| repository. A refusal names what is missing and never prints a value.
|
*/

package emailmatrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/process"
)

const envPrefix = "EMAIL_MATRIX"

/*
errUnknownFlag means the command line holds a flag the harness does not know.
*/
var errUnknownFlag = errors.New("emailmatrix: unknown flag")

/*
mailboxSecret holds one mailbox's credentials. Token is the API token of hosts
read over an API; the SMTP fields are for hosts that send over submission.
*/
type mailboxSecret struct {
	Address      string `json:"address"       koanf:"address"`
	SMTPHost     string `json:"smtp_host"     koanf:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"     koanf:"smtp_port"`
	SMTPUsername string `json:"smtp_username" koanf:"smtp_username"`
	SMTPPassword string `json:"smtp_password" koanf:"smtp_password"`
	Token        string `json:"token"         koanf:"token"`
}

/*
adapterTarget says where one provider adapter's mail goes: the domain it
receives on and the address row that collects what arrives.
*/
type adapterTarget struct {
	Domain    string `json:"domain"     koanf:"domain"`
	AddressID string `json:"address_id" koanf:"address_id"`
}

/*
settings is everything a run reads at start. BaseURL and APIToken reach the
staging application, Forwarding is the address mail is sent to, and AddressID
is the row that lists what arrived there. Root is where artifacts and recorded
payloads are written. SecretsID names the Secrets Manager document, whose fields
fill whatever the environment left empty. Args holds the flags.
*/
type settings struct {
	Args       string                   `json:"-"           koanf:"args"`
	Root       string                   `json:"-"           koanf:"root"`
	SecretsID  string                   `json:"-"           koanf:"secrets_id"`
	BaseURL    string                   `json:"base_url"    koanf:"base_url"`
	APIToken   string                   `json:"api_token"   koanf:"api_token"`
	Forwarding string                   `json:"forwarding"  koanf:"forwarding"`
	AddressID  string                   `json:"address_id"  koanf:"address_id"`
	Mailboxes  map[string]mailboxSecret `json:"mailboxes"   koanf:"mailboxes"`
	Adapters   map[string]adapterTarget `json:"adapters"    koanf:"adapters"`
}

/*
flags is the parsed command line. An empty Adapter runs the mailbox matrix.
*/
type flags struct {
	Adapter string
	Record  bool
}

/*
missingSecretsError refuses a run, naming the settings that are absent.
*/
type missingSecretsError struct {
	Names []string
}

func (e *missingSecretsError) Error() string {
	return "emailmatrix: refusing to run without " + strings.Join(e.Names, ", ")
}

/*
secretStore reads one secret document by name.
*/
type secretStore interface {
	Read(ctx context.Context, id string) ([]byte, error)
}

/*
awsCLISecrets reads Secrets Manager through the AWS command line, using
whatever credentials the operator's shell already has.
*/
type awsCLISecrets struct {
	runner *process.Process
}

func (s awsCLISecrets) Read(ctx context.Context, id string) ([]byte, error) {
	result, err := s.runner.Run(ctx, process.Cmd{
		Name: "aws",
		Args: []string{
			"secretsmanager", "get-secret-value", "--secret-id", id, "--query", "SecretString", "--output", "text",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("read secret document %s: %w", id, err)
	}

	return result.Stdout, nil
}

/*
loadSettings reads the settings and refuses when any credential the run needs
is absent. A nil environ reads the process environment.
*/
func loadSettings(ctx context.Context, environ []string, store secretStore) (*settings, flags, error) {
	var loaded settings

	opts := config.LoadOptions{Name: "emailmatrix", EnvPrefix: envPrefix, Environ: environ}
	if err := config.LoadInto(ctx, opts, &loaded); err != nil {
		return nil, flags{}, fmt.Errorf("load settings: %w", err)
	}

	parsed, err := parseFlags(loaded.Args)
	if err != nil {
		return nil, flags{}, err
	}

	if loaded.SecretsID != "" {
		if err := fillFromStore(ctx, &loaded, store); err != nil {
			return nil, flags{}, err
		}
	}

	if missing := loaded.missing(parsed); len(missing) > 0 {
		return nil, flags{}, &missingSecretsError{Names: missing}
	}

	return &loaded, parsed, nil
}

func fillFromStore(ctx context.Context, into *settings, store secretStore) error {
	document, err := store.Read(ctx, into.SecretsID)
	if err != nil {
		return err
	}

	var stored settings
	if err := json.Unmarshal(document, &stored); err != nil {
		return errors.New("secret document is not valid JSON")
	}

	into.BaseURL = cmpOr(into.BaseURL, stored.BaseURL)
	into.APIToken = cmpOr(into.APIToken, stored.APIToken)
	into.Forwarding = cmpOr(into.Forwarding, stored.Forwarding)
	into.AddressID = cmpOr(into.AddressID, stored.AddressID)

	if into.Mailboxes == nil {
		into.Mailboxes = stored.Mailboxes
	}

	if into.Adapters == nil {
		into.Adapters = stored.Adapters
	}

	return nil
}

func cmpOr(first, second string) string {
	if first != "" {
		return first
	}

	return second
}

/*
missing lists the settings the run cannot start without, by name only.
*/
func (s *settings) missing(run flags) []string {
	var names []string

	required := func(name, value string) {
		if value == "" {
			names = append(names, name)
		}
	}

	required("root", s.Root)
	required("base_url", s.BaseURL)
	required("api_token", s.APIToken)
	required("forwarding", s.Forwarding)
	required("address_id", s.AddressID)

	for _, host := range mailboxHosts() {
		for _, name := range s.Mailboxes[host.Name].missing(host) {
			names = append(names, "mailboxes."+host.Name+"."+name)
		}
	}

	if run.Adapter != "" && adapterByName(run.Adapter).Subdomain != "" {
		target := s.Adapters[run.Adapter]
		required("adapters."+run.Adapter+".domain", target.Domain)
		required("adapters."+run.Adapter+".address_id", target.AddressID)
	}

	slices.Sort(names)

	return names
}

func (m mailboxSecret) missing(host mailboxHost) []string {
	var names []string

	if m.Address == "" {
		names = append(names, "address")
	}

	if host.ReadsWithToken && m.Token == "" {
		names = append(names, "token")
	}

	if !host.SendsWithToken && (m.SMTPHost == "" || m.SMTPPassword == "") {
		names = append(names, "smtp_host and smtp_password")
	}

	return names
}

/*
parseFlags reads the command line the Makefile passes through, accepting one
or two leading dashes.
*/
func parseFlags(args string) (flags, error) {
	var parsed flags

	words := strings.Fields(args)
	for index := 0; index < len(words); index++ {
		name, value, hasValue := strings.Cut(strings.TrimLeft(words[index], "-"), "=")

		switch name {
		case "record":
			parsed.Record = true
		case "adapter":
			if !hasValue && index+1 == len(words) {
				return flags{}, fmt.Errorf("%w: adapter needs a name", errUnknownFlag)
			}

			if !hasValue {
				index++
				value = words[index]
			}

			if adapterByName(value).Name == "" {
				return flags{}, fmt.Errorf("%w: no adapter named %s", errUnknownFlag, value)
			}

			parsed.Adapter = value
		default:
			return flags{}, fmt.Errorf("%w: %s", errUnknownFlag, words[index])
		}
	}

	if parsed.Record && parsed.Adapter == "" {
		return flags{}, fmt.Errorf("%w: record needs an adapter", errUnknownFlag)
	}

	return parsed, nil
}
