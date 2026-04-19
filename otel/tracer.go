// Package otel wires OpenTelemetry tracing into claude-squad's headless
// server mode.
//
// Fork-only — does not exist in upstream smtg-ai/claude-squad. Activated
// only when `cs serve` is launched with OTEL env vars set; otherwise
// all functions are no-ops and claude-squad behaves exactly like
// upstream.
//
// The target deployment is Langfuse (self-hosted OTLP/HTTP ingestion)
// but the exporter is standard OTLP so any compatible collector works.
package otel

import (
	"claude-squad/log"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// ServiceName emitted on every span this package creates. Kept distinct
// from upstream Paperclip and from Claude Code's own spans so Langfuse
// filters show all three tiers cleanly.
const ServiceName = "claude-squad-server"

// Config controls the tracer setup.
type Config struct {
	Endpoint     string // e.g. http://localhost:3050/api/public/otel
	PublicKey    string // Langfuse public key (HTTP Basic username)
	SecretKey    string // Langfuse secret key (HTTP Basic password)
	Version      string // cs version, added as service.version resource attr
	Insecure     bool   // default true; disable only for HTTPS endpoints
}

// ConfigFromEnv reads the same env vars Paperclip already documents:
//
//	LANGFUSE_PUBLIC_KEY
//	LANGFUSE_SECRET_KEY
//	OTEL_EXPORTER_OTLP_ENDPOINT (or LANGFUSE_HOST)
//
// Returns (cfg, true) if enough env is present to enable; (zero, false)
// otherwise. Callers treat the false case as "tracing disabled".
func ConfigFromEnv(version string) (Config, bool) {
	pub := os.Getenv("LANGFUSE_PUBLIC_KEY")
	sec := os.Getenv("LANGFUSE_SECRET_KEY")
	if pub == "" || sec == "" {
		return Config{}, false
	}
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("LANGFUSE_HOST")
	}
	if endpoint == "" {
		endpoint = "http://localhost:3050/api/public/otel"
	}
	return Config{
		Endpoint:  endpoint,
		PublicKey: pub,
		SecretKey: sec,
		Version:   version,
		Insecure:  !strings.HasPrefix(endpoint, "https://"),
	}, true
}

// parseEndpoint returns (host:port, path) for Go's OTLP HTTP exporter,
// which wants them split. Accepts the usual three forms:
//
//	http://host:3050
//	http://host:3050/api/public/otel
//	http://host:3050/api/public/otel/v1/traces
func parseEndpoint(raw string) (host, path string, err error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", "", errors.New("empty endpoint")
	}
	// Split scheme
	scheme := ""
	switch {
	case strings.HasPrefix(u, "https://"):
		scheme = "https://"
	case strings.HasPrefix(u, "http://"):
		scheme = "http://"
	}
	u = strings.TrimPrefix(u, scheme)
	slash := strings.Index(u, "/")
	if slash < 0 {
		host = u
		path = "/api/public/otel/v1/traces"
	} else {
		host = u[:slash]
		path = u[slash:]
	}
	// Normalize the path to the /v1/traces endpoint.
	path = strings.TrimRight(path, "/")
	if !strings.HasSuffix(path, "/v1/traces") {
		if !strings.HasSuffix(path, "/api/public/otel") {
			path = path + "/api/public/otel"
		}
		path = path + "/v1/traces"
	}
	_ = scheme
	return host, path, nil
}

// Init installs a global tracer provider + W3C propagator. Returns a
// shutdown function; the caller invokes it on process exit to flush
// pending spans.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	host, path, err := parseEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid otel endpoint: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString(
		[]byte(cfg.PublicKey + ":" + cfg.SecretKey))

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithURLPath(path),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Basic " + auth,
		}),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("otlptrace exporter: %w", err)
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(ServiceName),
			semconv.ServiceVersion(cfg.Version),
		),
	)
	// Short export interval so short-lived instance.create spans flush
	// before the subprocess they instrument exits.
	bsp := sdktrace.NewBatchSpanProcessor(exp,
		sdktrace.WithBatchTimeout(1*time.Second),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	log.InfoLog.Printf("otel tracing enabled → %s%s (service=%s)",
		host, path, ServiceName)

	return func(shutdownCtx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 3*time.Second)
		defer cancel()
		return tp.Shutdown(shutdownCtx)
	}, nil
}

// Attrs is a convenience for building span attribute slices inline
// without pulling attribute.String / Int etc. into every call site.
type Attrs map[string]any

// Apply converts an Attrs map to the OTEL slice form.
func (a Attrs) Apply() []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(a))
	for k, v := range a {
		switch t := v.(type) {
		case string:
			out = append(out, attribute.String(k, t))
		case int:
			out = append(out, attribute.Int(k, t))
		case int64:
			out = append(out, attribute.Int64(k, t))
		case bool:
			out = append(out, attribute.Bool(k, t))
		case float64:
			out = append(out, attribute.Float64(k, t))
		default:
			out = append(out, attribute.String(k, fmt.Sprintf("%v", t)))
		}
	}
	return out
}
