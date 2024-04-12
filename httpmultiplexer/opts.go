package httpmultiplexer

import (
	"log/slog"
	"net/http"
	"time"
)

type Option func(c *HTTPMultiplexer)

func WithMaxURLs(v int) Option {
	return func(m *HTTPMultiplexer) {
		m.maxURLs = v
	}
}

func WithMaxReqPerOp(v int) Option {
	return func(m *HTTPMultiplexer) {
		m.maxReqPerOp = v
	}
}

func WithReqTimout(v time.Duration) Option {
	return func(m *HTTPMultiplexer) {
		m.reqTimeout = v
	}
}

func WithHTTPClient(v *http.Client) Option {
	return func(m *HTTPMultiplexer) {
		m.httpClient = v
	}
}

func WithLogger(v *slog.Logger) Option {
	return func(m *HTTPMultiplexer) {
		m.log = v
	}
}
