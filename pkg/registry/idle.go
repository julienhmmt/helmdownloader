package registry

import (
	"context"
	"io"
	"net/http"
	"time"
)

// pullIdleTimeout is how long a registry response may stay silent before the
// request is aborted. It bounds stalls, not total duration: a large layer
// that keeps sending is never cut off. Mutable var so tests can shorten it.
var pullIdleTimeout = 2 * time.Minute

// idleTimeoutTransport cancels a request whose response produces no bytes for
// idle. Without it a registry that stops sending hangs the pull forever and
// the retry logic never runs.
type idleTimeoutTransport struct {
	base http.RoundTripper
	idle time.Duration
}

func (t *idleTimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	timer := time.AfterFunc(t.idle, cancel)
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		timer.Stop()
		cancel()
		return nil, err
	}
	resp.Body = &idleBody{ReadCloser: resp.Body, timer: timer, idle: t.idle, cancel: cancel}
	return resp, nil
}

// idleBody pushes the deadline back on every read and releases the timer on
// close.
type idleBody struct {
	io.ReadCloser
	timer  *time.Timer
	idle   time.Duration
	cancel context.CancelFunc
}

func (b *idleBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.timer.Reset(b.idle)
	return n, err
}

func (b *idleBody) Close() error {
	b.timer.Stop()
	b.cancel()
	return b.ReadCloser.Close()
}
