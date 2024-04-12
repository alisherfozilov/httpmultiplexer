package httpmultiplexer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	defaultMaxURLs     = 20
	defaultMaxReqPerOp = 4
	defaultReqTimeout  = time.Second
)

type HTTPMultiplexer struct {
	maxURLs     int
	maxReqPerOp int
	reqTimeout  time.Duration
	log         *slog.Logger

	httpClient *http.Client
}

func New(opts ...Option) *HTTPMultiplexer {
	m := &HTTPMultiplexer{
		maxURLs:     defaultMaxURLs,
		maxReqPerOp: defaultMaxReqPerOp,
		reqTimeout:  defaultReqTimeout,
		log:         slog.Default(),
		httpClient:  http.DefaultClient,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

var (
	ErrTooManyURLs = errors.New("too many urls")
)

type ErrSlowURL struct {
	URL string
}

func (e ErrSlowURL) Error() string {
	return fmt.Sprintf("url %v failed to process the request within the specified timeout", e.URL)
}

type HTTPResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

func (m *HTTPMultiplexer) Multiplex(ctx context.Context, urls []string) ([]HTTPResponse, error) {
	if len(urls) > m.maxURLs {
		return nil, ErrTooManyURLs
	}
	results := make([]HTTPResponse, len(urls))

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(m.maxReqPerOp)
	for i, url := range urls {
		eg.Go(func() error {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("new request %v: %w", url, err)
			}
			ctx, cancel := context.WithTimeoutCause(ctx, m.reqTimeout, ErrSlowURL{URL: url})
			defer cancel()
			request.WithContext(ctx)

			response, err := m.httpClient.Do(request)
			if err != nil {
				return fmt.Errorf("do request %v: %w", url, err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				return fmt.Errorf("read response body: %w", err)
			}

			results[i] = HTTPResponse{
				StatusCode: response.StatusCode,
				Headers:    response.Header,
				Body:       body,
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("errgroup wait: %w", err)
	}
	return results, nil
}
