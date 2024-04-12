package httpmultiplexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type MultiplexRequest struct {
	URLs []string `json:"urls"`
}

type MultiplexResponse struct {
	HTTPResponses []MultiplexHTTPResponse `json:"http_responses"`
}

type MultiplexHTTPResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
}

func (m *HTTPMultiplexer) Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	var request MultiplexRequest
	if err := unmarshal(r, &request); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	httpResponses, err := m.Multiplex(r.Context(), request.URLs)
	if err != nil {
		m.handleError(w, r, err)
		return
	}
	response, err := toMultiplexResponse(httpResponses)
	if err != nil {
		m.log.Error("Internal error at handler", "url", r.URL.Path, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	m.reply(w, response)
}

func (m *HTTPMultiplexer) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrTooManyURLs) {
		http.Error(w, fmt.Sprintf("Validation: max number of urls is %v", m.maxURLs), http.StatusBadRequest)
		return
	}
	var e ErrSlowURL
	if errors.As(err, &e) {
		http.Error(w, fmt.Sprintf("URL %v timed out", e.URL), http.StatusUnprocessableEntity)
		return
	}

	m.log.Error("Internal error at handler", "url", r.URL.Path, "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	return
}

func toMultiplexResponse(rs []HTTPResponse) (MultiplexResponse, error) {
	response := MultiplexResponse{
		HTTPResponses: make([]MultiplexHTTPResponse, 0, len(rs)),
	}
	for _, r := range rs {
		mr := MultiplexHTTPResponse{
			StatusCode: r.StatusCode,
			Headers:    r.Headers,
			Body:       r.Body,
		}
		response.HTTPResponses = append(response.HTTPResponses, mr)
	}
	return response, nil
}

func unmarshal(r *http.Request, request *MultiplexRequest) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return fmt.Errorf("json unmarshal %s: %w", body, err)
	}
	return nil
}

func (m *HTTPMultiplexer) reply(w http.ResponseWriter, response any) {
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		m.log.Error("Error while sending response", "error", err)
	}
}
