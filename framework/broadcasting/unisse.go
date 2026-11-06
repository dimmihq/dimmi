/*
|--------------------------------------------------------------------------
| Unidirectional Server-Sent Events
|--------------------------------------------------------------------------
|
| Some clients only listen and cannot afford an SDK: a plain browser
| EventSource receives broadcasts here. EventSource cannot send headers
| or commands, so the whole connect command rides in the cf_connect
| query parameter: a short-lived token, never a session credential, and
| the last position of each channel so missed broadcasts are recovered.
| The server picks the channels from the principal.
|
| Each message is one event whose data is a JSON push: the connect
| result first, then publications, pings and finally the disconnect
| reason. A refused connection still answers 200 and ends with a
| disconnect push, which EventSource can read where a status code is
| invisible to it.
|
| A client with a token may listen from any site, since the token alone
| proves who it is: the answer allows the asking origin, but never with
| credentials, so a browser sends no cookie along. A client without one
| authenticates by its cookie and stays on the install's own site.
|
*/

package broadcasting

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/centrifugal/centrifuge"
)

/*
maxConnectParam caps the cf_connect parameter in bytes.
*/
const maxConnectParam = 16 << 10

/*
connectParam is the connect command an EventSource client sends.
*/
type connectParam struct {
	Token   string                    `json:"token"`
	Name    string                    `json:"name"`
	Version string                    `json:"version"`
	Subs    map[string]subscribeParam `json:"subs"`
}

/*
subscribeParam is the last position a client saw on a channel.
*/
type subscribeParam struct {
	Recover bool   `json:"recover"`
	Epoch   string `json:"epoch"`
	Offset  uint64 `json:"offset"`
}

/*
serveUniSSE streams pushes to one EventSource client until either side
closes the connection.
*/
func (s *Server) serveUniSSE(w http.ResponseWriter, r *http.Request) {
	req, err := parseConnect(r.URL.Query().Get("cf_connect"))
	if err != nil {
		http.Error(w, "invalid cf_connect parameter", http.StatusBadRequest)

		return
	}

	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Accel-Buffering", "no")
	header.Add("Vary", "Origin")

	if origin := r.Header.Get("Origin"); origin != "" && req.Token != "" {
		header.Set("Access-Control-Allow-Origin", origin)
	}
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	if err := rc.Flush(); err != nil {
		return
	}

	transport := newUniTransport(s.pingPong)

	client, closeClient, err := centrifuge.NewClient(r.Context(), s.node, transport)
	if err != nil {
		return
	}

	defer func() { _ = closeClient() }()

	go client.Connect(req)

	transport.pump(r, w, rc)
}

/*
parseConnect decodes the cf_connect parameter. A client may leave it
out when its credentials travel some other way, such as a cookie.
*/
func parseConnect(raw string) (centrifuge.ConnectRequest, error) {
	if len(raw) > maxConnectParam {
		return centrifuge.ConnectRequest{}, errors.New("cf_connect too long")
	}

	param := connectParam{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &param); err != nil {
			return centrifuge.ConnectRequest{}, err
		}
	}

	subs := make(map[string]centrifuge.SubscribeRequest, len(param.Subs))
	for channel, sub := range param.Subs {
		subs[channel] = centrifuge.SubscribeRequest{Recover: sub.Recover, Epoch: sub.Epoch, Offset: sub.Offset}
	}

	return centrifuge.ConnectRequest{Token: param.Token, Name: param.Name, Version: param.Version, Subs: subs}, nil
}

/*
uniTransport hands pushes from the client to the request's goroutine,
the only one allowed to write the response. A write returns once the
push is flushed or the transport is closed, so pushes keep their order
and a closed transport never blocks the client.
*/
type uniTransport struct {
	mu       sync.Mutex
	pingPong centrifuge.PingPongConfig
	frames   chan [][]byte
	written  chan struct{}
	closed   chan struct{}
	once     sync.Once
}

func newUniTransport(pingPong centrifuge.PingPongConfig) *uniTransport {
	return &uniTransport{
		pingPong: pingPong,
		frames:   make(chan [][]byte),
		written:  make(chan struct{}),
		closed:   make(chan struct{}),
	}
}

/*
Name names the transport in logs and metrics.
*/
func (t *uniTransport) Name() string { return "uni_sse" }

/*
AcceptProtocol leaves the HTTP version unreported.
*/
func (t *uniTransport) AcceptProtocol() string { return "" }

/*
Protocol is JSON, the only encoding EventSource can read.
*/
func (t *uniTransport) Protocol() centrifuge.ProtocolType { return centrifuge.ProtocolTypeJSON }

/*
ProtocolVersion is the current client protocol.
*/
func (t *uniTransport) ProtocolVersion() centrifuge.ProtocolVersion {
	return centrifuge.ProtocolVersion2
}

/*
Unidirectional is true: the client sends no commands.
*/
func (t *uniTransport) Unidirectional() bool { return true }

/*
Emulation is false: no second request carries commands.
*/
func (t *uniTransport) Emulation() bool { return false }

/*
DisabledPushFlags keeps every push, the disconnect one included.
*/
func (t *uniTransport) DisabledPushFlags() uint64 { return 0 }

/*
PingPongConfig sets how often the client is pinged.
*/
func (t *uniTransport) PingPongConfig() centrifuge.PingPongConfig { return t.pingPong }

/*
Write sends one push.
*/
func (t *uniTransport) Write(frame []byte) error {
	return t.WriteMany(frame)
}

/*
WriteMany sends pushes and waits until they are flushed.
*/
func (t *uniTransport) WriteMany(frames ...[]byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	select {
	case t.frames <- frames:
	case <-t.closed:
		return nil
	}

	select {
	case <-t.written:
	case <-t.closed:
	}

	return nil
}

/*
Close releases waiting writers and ends the stream.
*/
func (t *uniTransport) Close(centrifuge.Disconnect) error {
	t.once.Do(func() { close(t.closed) })

	return nil
}

/*
pump writes pushes as server-sent events until the transport closes,
the client goes away or a write fails. Leaving closes the transport, so a
writer still waiting to hand over a push is released instead of holding
the client's shutdown forever.
*/
func (t *uniTransport) pump(r *http.Request, w http.ResponseWriter, rc *http.ResponseController) {
	defer t.once.Do(func() { close(t.closed) })

	for {
		select {
		case frames := <-t.frames:
			if writeEvents(w, rc, frames) != nil {
				return
			}

			select {
			case t.written <- struct{}{}:
			case <-t.closed:
				return
			}
		case <-t.closed:
			return
		case <-r.Context().Done():
			return
		}
	}
}

/*
writeEvents writes each push as one event and flushes them together.
*/
func writeEvents(w http.ResponseWriter, rc *http.ResponseController, frames [][]byte) error {
	for _, frame := range frames {
		event := make([]byte, 0, len(frame)+8)
		event = append(event, "data: "...)
		event = append(event, frame...)
		event = append(event, "\n\n"...)

		if _, err := w.Write(event); err != nil {
			return err
		}
	}

	return rc.Flush()
}
