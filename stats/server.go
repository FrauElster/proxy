package stats

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/FrauElster/proxy"
	"github.com/FrauElster/proxy/internal"
)

//go:embed static/*
var staticFiles embed.FS

type enhancedRec struct {
	StatRecorder
	resStarted time.Time
}

type StatServer struct {
	captureWindow   time.Duration
	targetRecorders map[string]*enhancedRec
	port            int
}

type StatServerOption func(*StatServer)

func WithPort(port int) StatServerOption {
	return func(s *StatServer) { s.port = port }
}

func WithCaptureWindow(window time.Duration) StatServerOption {
	return func(s *StatServer) { s.captureWindow = window }
}

func NewStatServer(opts ...StatServerOption) *StatServer {
	s := &StatServer{
		port:            8081,
		captureWindow:   2 * time.Minute,
		targetRecorders: make(map[string]*enhancedRec),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *StatServer) RegisterTarget(target proxy.Target) {
	s.targetRecorders[target.Prefix] = &enhancedRec{StatRecorder: *newStatRecorder(s.captureWindow)}
	target.PreRequest = s.PreRequest(target.Prefix)
	target.PostRequest = s.PostRequest(target.Prefix)
}

func (s *StatServer) PreRequest(targetPrefix string) func(*http.Request) *http.Request {
	rec, ok := s.targetRecorders[targetPrefix]
	if !ok {
		return nil
	}

	return func(r *http.Request) *http.Request {
		rec.resStarted = time.Now()
		return r
	}
}

func (s *StatServer) PostRequest(targetPrefix string) func(*http.Response) *http.Response {
	rec, ok := s.targetRecorders[targetPrefix]
	if !ok {
		return nil
	}

	return func(r *http.Response) *http.Response {
		status := http.StatusBadGateway
		if r != nil {
			status = r.StatusCode
		}
		rec.AddResponse(time.Since(rec.resStarted), status)
		return r
	}
}

func (s *StatServer) ListenAndServe() error {
	// serve index.html and index.js
	staticServer := http.FileServer(http.FS(staticFiles))
	http.Handle("/static/", staticServer)

	// serve targets data
	apiPrefix := "/api"
	http.HandleFunc(internal.JoinUrl(apiPrefix, "targets"), func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Targets []string `json:"targets"`
		}{Targets: mapKeys(s.targetRecorders)}
		sendJson(w, data)
	})
	for name, target := range s.targetRecorders {
		http.HandleFunc(internal.JoinUrl(apiPrefix, "targets", name), handleTargetRequest(&target.StatRecorder))
	}

	server := &http.Server{Addr: fmt.Sprintf(":%d", s.port)}

	slog.Info("Starting stats server", "port", s.port)
	return server.ListenAndServe()
}

func handleTargetRequest(recorder *StatRecorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { sendJson(w, recorder.GetStat()) }
}

func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func sendJson(w http.ResponseWriter, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		jsonData, _ = json.Marshal(jsonError{Error: err.Error()})
		http.Error(w, string(jsonData), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

type jsonError struct {
	Error string `json:"error"`
}
