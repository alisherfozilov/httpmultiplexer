package httpmultiplexer

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPMultiplexer_Multiplex(t *testing.T) {
	ctx := context.Background()
	const maxReq = 2
	m := New(WithMaxReqPerOp(maxReq))

	serverErr := make(chan error, 1)
	var reqs atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check WithMaxReqPerOp setting
		reqs.Add(1)
		time.Sleep(3 * time.Millisecond)
		rn := reqs.Load()
		if rn > maxReq {
			trySend(serverErr, fmt.Errorf("maximum number of requests %v limit exceeded: %v", maxReq, rn))
		}
		reqs.Add(-1)

		param := r.URL.Query().Get("param")
		w.Header().Set("Content-Type", "text/plain")
		w.Header()["Date"] = nil
		_, err := w.Write([]byte(param))
		if err != nil {
			t.Log(err)
		}
	}))
	defer server.Close()

	var urls []string
	var want []HTTPResponse
	for i := 1; i <= 10; i++ {
		urls = append(urls, fmt.Sprintf("%v?param=%v", server.URL, i))
		want = append(want, HTTPResponse{
			StatusCode: 200,
			Headers: map[string][]string{
				"Content-Length": {strconv.Itoa(len(strconv.Itoa(i)))},
				"Content-Type":   {"text/plain"},
			},
			Body: []byte(strconv.Itoa(i)),
		})
	}

	got, err := m.Multiplex(ctx, urls)
	select {
	case e := <-serverErr:
		t.Fatal(e)
	default:
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Fatalf("len(want) %v != len(got) %v", len(want), len(got))
	}
	for i := range want {
		if want[i].StatusCode != got[i].StatusCode {
			t.Fatalf("Wrong status codes %v != %v", want[i].StatusCode, got[i].StatusCode)
		}
		if !reflect.DeepEqual(want[i].Headers, got[i].Headers) {
			t.Fatalf("Wrong headers %v != %v", want[i].Headers, got[i].Headers)
		}
		if !bytes.Equal(want[i].Body, got[i].Body) {
			t.Fatalf("Wrong bodies %s != %s", want[i].Body, got[i].Body)
		}
	}
}

func trySend[T any](ch chan T, a T) {
	select {
	case ch <- a:
	default:
	}
}
