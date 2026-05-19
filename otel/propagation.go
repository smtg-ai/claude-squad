package otel

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceparentFromContext returns the W3C traceparent / tracestate for
// the active span on ctx, suitable for injection as env vars or HTTP
// headers. Returns empty strings when tracing is disabled or the
// context has no active span.
func TraceparentFromContext(ctx context.Context) (traceparent, tracestate string) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.Get("traceparent"), carrier.Get("tracestate")
}

// SubprocessEnv builds the env-var slice to pass to exec.Cmd.Env so a
// spawned agent subprocess (claude, codex, aider, gemini, ...) parents
// its OTEL spans under the active span on ctx and exports to the same
// Langfuse instance.
//
// The returned slice contains ONLY OTEL-related vars. Merge with the
// parent's os.Environ() at the call site.
//
// `uniqueServiceName` lets the caller differentiate the agent from the
// cs-server itself in Langfuse; pass e.g. "cs-agent-<instance-id>".
func SubprocessEnv(ctx context.Context, cfg Config, uniqueServiceName string) []string {
	if cfg.PublicKey == "" || cfg.SecretKey == "" {
		return nil
	}
	tp, ts := TraceparentFromContext(ctx)

	auth := base64.StdEncoding.EncodeToString(
		[]byte(cfg.PublicKey + ":" + cfg.SecretKey))
	endpointBase := cfg.Endpoint
	// Children should receive the base path, not /v1/traces; the OTLP
	// exporter in the child appends the signal-specific path itself.
	if host, _, err := parseEndpoint(endpointBase); err == nil {
		// Always hand children the /api/public/otel form; the child's
		// OTEL_EXPORTER_OTLP_ENDPOINT is interpreted as a base URL.
		scheme := "http"
		if !cfg.Insecure {
			scheme = "https"
		}
		endpointBase = fmt.Sprintf("%s://%s/api/public/otel", scheme, host)
	}

	env := []string{
		"CLAUDE_CODE_ENABLE_TELEMETRY=1",
		"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1",
		"OTEL_TRACES_EXPORTER=otlp",
		"OTEL_METRICS_EXPORTER=otlp",
		"OTEL_LOGS_EXPORTER=otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
		"OTEL_EXPORTER_OTLP_ENDPOINT=" + endpointBase,
		"OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic " + auth,
		"OTEL_TRACES_EXPORT_INTERVAL=1000",
		"OTEL_METRIC_EXPORT_INTERVAL=5000",
		"OTEL_LOGS_EXPORT_INTERVAL=1000",
	}
	if uniqueServiceName != "" {
		env = append(env, "OTEL_SERVICE_NAME="+uniqueServiceName)
	}
	if tp != "" {
		env = append(env, "TRACEPARENT="+tp)
	}
	if ts != "" {
		env = append(env, "TRACESTATE="+ts)
	}
	// Keep Anthropic credentials flowing through, so the caller's
	// merge-with-environ handoff stays consistent.
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		env = append(env, "ANTHROPIC_API_KEY="+v)
	}
	if v := os.Getenv("ANTHROPIC_BASE_URL"); v != "" {
		env = append(env, "ANTHROPIC_BASE_URL="+v)
	}
	if v := os.Getenv("ANTHROPIC_AUTH_TOKEN"); v != "" {
		env = append(env, "ANTHROPIC_AUTH_TOKEN="+v)
	}
	return env
}

// TracerFor returns a tracer scoped to a logical subsystem (e.g.
// "claude-squad-server"). Callers should hold onto this and reuse it.
func TracerFor(name string) trace.Tracer {
	return otel.Tracer(name)
}
