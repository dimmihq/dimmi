/*
|--------------------------------------------------------------------------
| Roles and Listeners
|--------------------------------------------------------------------------
|
| Starting the application starts its roles: the queue for workers and
| the scheduler, then the listeners for web. The public listener serves
| the application; the admin listener, on loopback by default, serves
| health and metrics to the machine's own tooling and never to the
| internet. Both answer liveness and readiness, so a load balancer and
| an orchestrator can each use the one they can reach.
|
| Shutdown runs in reverse within its budget: the listeners stop
| accepting and finish their requests, realtime clients are told to
| reconnect elsewhere, and the workers finish their jobs before the
| pools close.
|
*/

package foundation

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/dimmihq/dimmi/framework/health"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
Health endpoint paths, served on both listeners.
*/
const (
	PathUp      = "/up"
	PathReady   = "/ready"
	PathMetrics = "/metrics"
)

/*
webServers are the addresses of the running listeners and the errors
they stop with.
*/
type webServers struct {
	public string
	secure string
	admin  string
	errs   chan error
}

/*
Start starts the process's roles and returns once they run: the queue
for workers and the scheduler, then the listeners for web.
*/
func (a *Application) Start(ctx context.Context) error {
	if a.Roles.Has(RoleWorker) || a.Roles.Has(RoleScheduler) {
		if err := a.startQueue(ctx); err != nil {
			return err
		}
	}

	if a.Roles.Has(RoleWeb) {
		return a.startWeb(ctx)
	}

	return a.startAdmin(ctx)
}

/*
startAdmin serves health and metrics on the admin listener for a process
without the web role, so a worker can be checked like a web server.
*/
func (a *Application) startAdmin(ctx context.Context) error {
	if a.Config.HTTP.AdminAddr == "" {
		return nil
	}

	web := &webServers{errs: make(chan error, 1)}

	addr, err := a.listen(ctx, endpoint{"admin", a.Config.HTTP.AdminAddr, a.adminHandler(), nil}, web.errs)
	if err != nil {
		return err
	}

	web.admin = addr
	a.web.Store(web)

	return nil
}

/*
Serve starts the roles and runs until the context ends or a listener
fails, then shuts down within the configured timeout.
*/
func (a *Application) Serve(ctx context.Context) error {
	err := a.Start(ctx)
	if err == nil {
		a.Log.InfoContext(ctx, "serving", log.Any("roles", a.Roles))
		err = a.wait(ctx)
	}

	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.Config.HTTP.ShutdownTimeout)
	defer cancel()

	return errors.Join(err, a.Shutdown(stopCtx))
}

/*
Addr returns the address the public listener is bound to, or "" before
it starts.
*/
func (a *Application) Addr() string {
	if web := a.web.Load(); web != nil {
		return web.public
	}

	return ""
}

/*
TLSAddr returns the address the HTTPS listener is bound to, or "" when
the application serves plain HTTP only or has not started.
*/
func (a *Application) TLSAddr() string {
	if web := a.web.Load(); web != nil {
		return web.secure
	}

	return ""
}

/*
AdminAddr returns the address the admin listener is bound to, or ""
when it is off or not started.
*/
func (a *Application) AdminAddr() string {
	if web := a.web.Load(); web != nil {
		return web.admin
	}

	return ""
}

/*
wait blocks until the context ends or a listener fails.
*/
func (a *Application) wait(ctx context.Context) error {
	web := a.web.Load()
	if web == nil {
		<-ctx.Done()

		return nil
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-web.errs:
		return err
	}
}

/*
startQueue applies the schedule and starts the queue. Jobs run on a
context that outlives the caller's, so a signal does not cut them off:
shutdown stops the queue within its budget instead.
*/
func (a *Application) startQueue(ctx context.Context) error {
	if err := a.Schedule.Apply(); err != nil {
		return fmt.Errorf("apply schedule: %w", err)
	}

	if err := a.Queue.Start(log.NewContext(context.WithoutCancel(ctx), a.Log)); err != nil {
		return err
	}

	a.onClose("queue workers", a.Queue.Stop)

	return nil
}

/*
startWeb binds the public and admin listeners and serves them. With an
HTTPS address the application terminates TLS itself and the plain
listener only answers health, challenges and redirects. Realtime clients
are told to reconnect before the listeners wait for requests to finish,
so long-lived streams never hold shutdown up.
*/
func (a *Application) startWeb(ctx context.Context) error {
	public, err := a.publicHandler()
	if err != nil {
		return err
	}

	web := &webServers{errs: make(chan error, 3)}
	stopBroadcasting := a.stopBroadcasting()
	a.onClose("broadcasting connections", stopBroadcasting)

	if err := a.listenPublic(ctx, web, public); err != nil {
		return err
	}

	if a.Config.HTTP.AdminAddr != "" {
		web.admin, err = a.listen(ctx, endpoint{"admin", a.Config.HTTP.AdminAddr, a.adminHandler(), nil}, web.errs)
		if err != nil {
			return err
		}
	}

	a.onClose("realtime clients", stopBroadcasting)
	a.web.Store(web)

	return nil
}

/*
listenPublic serves the application on the plain address, or on the
HTTPS address with the plain one redirecting to it.
*/
func (a *Application) listenPublic(ctx context.Context, web *webServers, public http.Handler) error {
	var err error

	if a.Config.HTTP.TLSAddr == "" {
		web.public, err = a.listen(ctx, endpoint{"public", a.Config.HTTP.Addr, public, nil}, web.errs)

		return err
	}

	secure := a.Domains.TLSConfig()
	if secure == nil {
		return ErrNoTLSDriver
	}

	web.secure, err = a.listen(ctx, endpoint{"https", a.Config.HTTP.TLSAddr, strictTransport(public), secure}, web.errs)
	if err != nil {
		return err
	}

	web.public, err = a.listen(ctx, endpoint{"http", a.Config.HTTP.Addr, a.redirectHandler(), nil}, web.errs)

	return err
}

/*
publicHandler serves health, realtime connections and the application's
routes through the kernel, with certificate challenges answered first.
*/
func (a *Application) publicHandler() (http.Handler, error) {
	routes, err := a.Router.Handler()
	if err != nil {
		return nil, fmt.Errorf("build routes: %w", err)
	}

	mux := a.healthMux()
	for path, handler := range a.Broadcast.Handlers() {
		mux.Handle(path, handler)
	}

	mux.Handle("/", a.Kernel.Handler(routes))

	return a.Domains.HTTPHandler(mux), nil
}

/*
adminHandler serves health and metrics.
*/
func (a *Application) adminHandler() http.Handler {
	mux := a.healthMux()
	mux.Handle(PathMetrics, a.Telemetry.MetricsHandler())

	return mux
}

func (a *Application) healthMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(PathUp, health.Up(a.Config.App.Release))
	mux.Handle(PathReady, health.Ready(a.Health))

	return mux
}

/*
endpoint is a listener to start: its name in messages, the address to
bind, the handler to serve and, for HTTPS, its TLS configuration.
*/
type endpoint struct {
	name    string
	addr    string
	handler http.Handler
	tls     *tls.Config
}

/*
listen binds the endpoint's address and serves its handler until
shutdown, which stops accepting and waits for requests in flight. It
returns the bound address; a failure while serving is sent to errs.
*/
func (a *Application) listen(ctx context.Context, e endpoint, errs chan<- error) (string, error) {
	listener, err := new(net.ListenConfig).Listen(ctx, "tcp", e.addr)
	if err != nil {
		return "", fmt.Errorf("listen %s on %s: %w", e.name, e.addr, err)
	}

	server := newServer(log.NewContext(context.WithoutCancel(ctx), a.Log), e.handler, a.Config.HTTP)
	server.TLSConfig = e.tls
	a.onClose(e.name+" listener", server.Shutdown)

	go func() {
		if err := serve(server, listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- fmt.Errorf("serve %s: %w", e.name, err)
		}
	}()

	return listener.Addr().String(), nil
}

/*
serve serves plain HTTP, or HTTPS when the server has a TLS
configuration. ServeTLS is what turns HTTP/2 on; its certificates come
from the configuration, so no files are named.
*/
func serve(server *http.Server, listener net.Listener) error {
	if server.TLSConfig == nil {
		return server.Serve(listener)
	}

	return server.ServeTLS(listener, "", "")
}

/*
newServer returns the HTTP server for a listener. Request contexts
derive from the base context, so they carry the application's logger.
*/
func newServer(base context.Context, handler http.Handler, cfg HTTPConfig) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		BaseContext:       func(net.Listener) context.Context { return base },
	}
}
