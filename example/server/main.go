package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/netutil"

	"httpmultiplexer/httpmultiplexer"
)

const (
	maxConcurrentHTTPReq = 100
	address              = "0.0.0.0:8080"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	httpClient := newHTTPClient()

	m := httpmultiplexer.New(
		httpmultiplexer.WithMaxURLs(20),
		httpmultiplexer.WithMaxReqPerOp(4),
		httpmultiplexer.WithReqTimout(time.Second),
		httpmultiplexer.WithHTTPClient(httpClient),
	)

	router := http.NewServeMux()
	router.HandleFunc("/multiplex", m.Handler)

	server := &http.Server{Addr: address, Handler: router}

	errCh := make(chan error, 1)
	go func() {
		if err := listenAndServe(server); err != nil {
			errCh <- err
		}
	}()
	select {
	case s := <-exit():
		log.Println("Service got signal", s)
	case err := <-errCh:
		log.Println("Service got fatal error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}

func listenAndServe(server *http.Server) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("net listen: %w", err)
	}
	listener = netutil.LimitListener(listener, maxConcurrentHTTPReq)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func exit() chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	return ch
}

func newHTTPClient() *http.Client {
	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport.IdleConnTimeout = time.Minute
	httpTransport.MaxIdleConnsPerHost = 4
	httpTransport.MaxConnsPerHost = 8
	httpClient := &http.Client{Transport: http.DefaultTransport}
	return httpClient
}
