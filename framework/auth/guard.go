/*
|--------------------------------------------------------------------------
| Guards
|--------------------------------------------------------------------------
|
| A guard reads one kind of credential from a request: a session cookie,
| a bearer token. What kind of caller a request comes from is decided by
| looking the credential up, never by anything the client claims, so the
| guards run in the order they were registered and the first one that
| finds its credential decides. Registering bearer guards before the
| session guard makes an explicit token win over an ambient cookie.
|
| A route names the kinds of caller it accepts. A caller of another kind
| is not a stranger, so it gets 403 with a code that says what this route
| needs instead of 401, and a route may name the refusal it gives.
|
*/

package auth

import (
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
ErrNoCredentials is returned by a guard or token resolver when the
request carries no credential of its kind, so the next one may look.
*/
var ErrNoCredentials = errors.New("auth: no credentials")

/*
ErrUnknownGuard is returned when a route names a guard or set of guards
that was never registered or defined.
*/
var ErrUnknownGuard = errors.New("auth: unknown guard")

/*
The faults guards answer with.

ErrUnauthenticated means no guard found a credential. ErrInvalidToken
means a bearer token is not one the application issued.
ErrCredentialRefused means the caller is known but the route does not
accept their kind of credential. ErrMissingAbility means a token lacks
an ability the route requires; the response names it in
required_ability.
*/
var (
	ErrUnauthenticated   = fault.New(fault.Unauthenticated, "unauthenticated", "Unauthenticated.")
	ErrInvalidToken      = fault.New(fault.Unauthenticated, "invalid_token", "The access token is invalid.")
	ErrCredentialRefused = fault.New(fault.Forbidden, "credential_not_accepted",
		"This route does not accept this kind of credential.")
)

/*
Guard authenticates requests that carry its kind of credential. It
returns ErrNoCredentials when there is none, and a fault such as
ErrInvalidToken when there is one that does not check out.
*/
type Guard interface {
	/*
		Authenticate returns the caller the request's credential belongs to.
	*/
	Authenticate(r *http.Request) (Principal, error)
}

/*
GuardFunc adapts a function to a Guard.
*/
type GuardFunc func(r *http.Request) (Principal, error)

/*
Authenticate calls the function.
*/
func (f GuardFunc) Authenticate(r *http.Request) (Principal, error) {
	return f(r)
}

/*
Any returns a guard that asks each guard in turn and answers with the
first one that finds its credential.
*/
func Any(guards ...Guard) Guard {
	return GuardFunc(func(r *http.Request) (Principal, error) {
		for _, guard := range guards {
			p, err := guard.Authenticate(r)
			if errors.Is(err, ErrNoCredentials) {
				continue
			}

			return p, err
		}

		return nil, ErrNoCredentials
	})
}

/*
SessionGuard returns the guard for the logged-in user of the request's
session, which the session middleware must have loaded. The principal
holds every ability and carries the session's last confirmation.
*/
func SessionGuard(name string) Guard {
	return GuardFunc(func(r *http.Request) (Principal, error) {
		s, ok := session.From(r.Context())
		if !ok {
			return nil, ErrNoCredentials
		}

		user, ok := s.UserID()
		if !ok {
			return nil, ErrNoCredentials
		}

		return &Identity{
			User:       user,
			Via:        name,
			Credential: s.ID(),
			Granted:    []string{AllAbilities},
			Confirmed:  s.ConfirmedAt(),
		}, nil
	})
}

/*
accepted is what a route parameter of the auth middleware stands for:
the kinds of caller it lets in and the fault every other kind gets.
*/
type accepted struct {
	guards  []string
	refusal *fault.Error
}

/*
Guards holds the registered guards, in the order they run, and the named
sets of caller kinds routes accept.
*/
type Guards struct {
	order  []string
	guards map[string]Guard
	sets   map[string]accepted
	errors http.Errors
}

/*
NewGuards returns an empty registry whose middleware renders failures
with the errors renderer.
*/
func NewGuards(errs http.Errors) *Guards {
	return &Guards{guards: map[string]Guard{}, sets: map[string]accepted{}, errors: errs}
}

/*
Register adds a guard, run after those registered before it. The name
also becomes a route parameter accepting principals whose Guard is that
name, unless a set of that name is defined.
*/
func (g *Guards) Register(name string, guard Guard) {
	if _, ok := g.guards[name]; !ok {
		g.order = append(g.order, name)
	}

	g.guards[name] = guard

	if _, ok := g.sets[name]; !ok {
		g.sets[name] = accepted{guards: []string{name}, refusal: ErrCredentialRefused}
	}
}

/*
Define names a set of caller kinds a route can accept, such as "user"
for "session" and "app", and the fault callers of any other kind get.
A nil refusal falls back to ErrCredentialRefused. The kinds are matched
against the principal's Guard, so a token resolver deciding between two
kinds by lookup can serve two sets through one guard.
*/
func (g *Guards) Define(name string, refusal *fault.Error, kinds ...string) {
	if refusal == nil {
		refusal = ErrCredentialRefused
	}

	g.sets[name] = accepted{guards: slices.Clone(kinds), refusal: refusal}
}

/*
Authenticate runs the guards in order and returns the caller found by
the first guard that sees its credential, or ErrUnauthenticated when no
guard does.
*/
func (g *Guards) Authenticate(r *http.Request) (Principal, error) {
	for _, name := range g.order {
		p, err := g.guards[name].Authenticate(r)
		if errors.Is(err, ErrNoCredentials) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("authenticate with %s guard: %w", name, err)
		}

		return p, nil
	}

	return nil, ErrUnauthenticated
}

/*
Middleware builds the authentication middleware for a route, registered
as a kernel alias such as "auth". Each parameter names a guard or a
defined set; "auth:session" lets in session callers only and "auth"
alone any caller. The middleware also refuses a caller whose credential
lacks an ability the matched route requires. The principal goes into
the request context, and so does its tenant when it has one.
*/
func (g *Guards) Middleware(params ...string) (http.Middleware, error) {
	want, err := g.accepting(params)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := g.admit(r, want)
			if err != nil {
				g.errors.Render(w, r, err)

				return
			}

			ctx := http.WithPrincipal(r.Context(), p)
			if tenant := p.Tenant(); tenant != "" {
				ctx = tenancy.With(ctx, tenant)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

/*
accepting merges the sets the parameters name. No parameters accepts
every kind of caller, which a nil list stands for.
*/
func (g *Guards) accepting(params []string) (accepted, error) {
	var want accepted

	for _, param := range params {
		set, ok := g.sets[param]
		if !ok {
			return accepted{}, fmt.Errorf("%w: %q", ErrUnknownGuard, param)
		}

		want.guards = append(want.guards, set.guards...)
		if want.refusal == nil {
			want.refusal = set.refusal
		}
	}

	if len(params) > 0 && want.guards == nil {
		want.guards = []string{}
	}

	return want, nil
}

/*
admit authenticates the request and checks the caller against what the
route accepts and the abilities it requires.
*/
func (g *Guards) admit(r *http.Request, want accepted) (Principal, error) {
	p, err := g.Authenticate(r)
	if err != nil {
		return nil, err
	}

	if want.guards != nil && !slices.Contains(want.guards, p.Guard()) {
		return nil, want.refusal
	}

	if err := checkAbilities(r.Context(), p); err != nil {
		return nil, err
	}

	return p, nil
}
