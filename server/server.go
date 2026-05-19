package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"claude-squad/log"
	otelpkg "claude-squad/otel"

	"go.opentelemetry.io/otel/trace"
)

// Options configures the HTTP server.
type Options struct {
	// Addr to bind, e.g. ":3200" or "127.0.0.1:3200".
	Addr string
	// AuthToken is required in the `Authorization: Bearer <token>`
	// header on every request except /v1/health. When empty, auth is
	// disabled — intended for local dev only.
	AuthToken string
	// Version is echoed in the /v1/health response.
	Version string
	// OtelCfg, when its PublicKey + SecretKey are non-empty, activates
	// per-instance OTEL span emission + TRACEPARENT injection into the
	// spawned agent subprocess. Empty = tracing disabled (handlers
	// still work, just without instrumentation).
	OtelCfg otelpkg.Config
}

// Server is the runtime wrapper.
type Server struct {
	opts    Options
	store   *store
	bus     *eventBus
	httpSrv *http.Server
	tracer  trace.Tracer
}

// New constructs a server but does not bind the socket. Call Serve().
func New(opts Options) *Server {
	if opts.Addr == "" {
		opts.Addr = ":3200"
	}
	if opts.Version == "" {
		opts.Version = "unknown"
	}
	s := &Server{
		opts:   opts,
		store:  newStore(),
		bus:    newEventBus(),
		tracer: otelpkg.TracerFor(otelpkg.ServiceName),
	}
	mux := http.NewServeMux()
	s.register(mux)
	s.httpSrv = &http.Server{
		Addr:              opts.Addr,
		Handler:           s.authMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

// Serve starts the server and blocks until ctx is canceled or a
// SIGINT/SIGTERM arrives. Listen errors are returned to the caller.
func (s *Server) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		log.InfoLog.Printf("cs serve listening on %s", s.opts.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		log.InfoLog.Printf("cs serve shutting down on %s", sig)
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(shutdownCtx)
}

// Bus returns the server's event bus so future code can subscribe to
// lifecycle events without routing them through HTTP.
func (s *Server) Bus() *eventBus { return s.bus }

// -------- middleware --------

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/health" || s.opts.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			writeErr(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		supplied := strings.TrimSpace(h[len("bearer "):])
		if supplied != s.opts.AuthToken {
			writeErr(w, http.StatusUnauthorized, "invalid bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// -------- response helpers --------

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, ErrorResponse{Error: msg})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
