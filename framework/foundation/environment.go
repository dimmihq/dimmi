/*
|--------------------------------------------------------------------------
| Environment and Roles
|--------------------------------------------------------------------------
|
| The environment says what kind of run this is. Only the testing
| environment may swap services for fakes, and only production guards
| destructive commands behind a confirmation. Roles say what a process
| does: serve HTTP, work jobs, or schedule periodic jobs. Every role may
| run in one process, or each in processes of its own.
|
*/

package foundation

import (
	"errors"
	"fmt"
	"slices"
)

/*
Environment is the kind of run: local development, the test suite or
production.
*/
type Environment string

/*
The environments, described on Environment.
*/
const (
	EnvLocal      Environment = "local"
	EnvTesting    Environment = "testing"
	EnvProduction Environment = "production"
)

/*
Role is one job a process does.

RoleWeb serves the public and admin listeners. RoleWorker runs queued
jobs. RoleScheduler pushes periodic jobs from whichever node holds the
queue's leadership.
*/
type Role string

/*
The roles, described on Role.
*/
const (
	RoleWeb       Role = "web"
	RoleWorker    Role = "worker"
	RoleScheduler Role = "scheduler"
)

/*
Roles is the set of roles a process runs.
*/
type Roles []Role

/*
Errors from checking the environment and roles.
*/
var (
	ErrInvalidEnvironment = errors.New("foundation: environment must be local, testing or production")
	ErrInvalidRole        = errors.New("foundation: role must be web, worker or scheduler")
)

/*
Valid reports whether the environment is one we know.
*/
func (e Environment) Valid() bool {
	switch e {
	case EnvLocal, EnvTesting, EnvProduction:
		return true
	}

	return false
}

/*
Has reports whether the role is in the set.
*/
func (r Roles) Has(role Role) bool {
	return slices.Contains(r, role)
}

/*
validate returns ErrInvalidRole for the first role that is not one we
know.
*/
func (r Roles) validate() error {
	for _, role := range r {
		switch role {
		case RoleWeb, RoleWorker, RoleScheduler:
			continue
		}

		return fmt.Errorf("%w: %q", ErrInvalidRole, role)
	}

	return nil
}
