package gax

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"testing"

	"github.com/googleapis/gax-go/v2/callctx"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type mockHandler struct {
	records []slog.Record
}

func (m *mockHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (m *mockHandler) Handle(ctx context.Context, r slog.Record) error {
	m.records = append(m.records, r)
	return nil
}

func (m *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return m
}

func (m *mockHandler) WithGroup(name string) slog.Handler {
	return m
}

func TestInvokeWithLogging(t *testing.T) {
	t.Setenv("GOOGLE_SDK_GO_EXPERIMENTAL_TRACING", "true")
	TestOnlyResetIsFeatureEnabled()
	defer TestOnlyResetIsFeatureEnabled()

	handler := &mockHandler{}
	logger := slog.New(handler)

	ctx := context.Background()
	ctx = callctx.WithTelemetryContext(ctx, "rpc_method", "my.test.Method", "url_template", "/v1/foo?query=123")

	opts := []LoggingOption{
		WithLoggerProvider(logger),
		WithLoggingAttributes(map[string]string{
			URLDomain: "test.domain",
			RPCSystem: "grpc",
		}),
	}
	cl := NewClientLogging(opts...)
	callOpts := []CallOption{WithClientLogging(cl)}

	expectedErr := &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("simulated error")}
	callFunc := func(ctx context.Context, settings CallSettings) error {
		return expectedErr
	}

	err := Invoke(ctx, callFunc, callOpts...)
	if err == nil {
		t.Fatalf("Invoke() expected error")
	}

	if len(handler.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(handler.records))
	}
	record := handler.records[0]

	if record.Message != "simulated error" {
		t.Errorf("expected stripped url error message 'simulated error', got %q", record.Message)
	}

	if record.Level != slog.LevelError {
		t.Errorf("expected level error, got %v", record.Level)
	}

	var foundDomain, foundSystem, foundMethod, foundTemplate bool
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "url.domain" && attr.Value.String() == "test.domain" {
			foundDomain = true
		}
		if attr.Key == "rpc.system.name" && attr.Value.String() == "grpc" {
			foundSystem = true
		}
		if attr.Key == "rpc.method" && attr.Value.String() == "my.test.Method" {
			foundMethod = true
		}
		if attr.Key == "url.template" && attr.Value.String() == "/v1/foo" {
			foundTemplate = true
		}
		return true
	})

	if !foundDomain || !foundSystem || !foundMethod || !foundTemplate {
		t.Errorf("missing expected attributes: domain=%v system=%v method=%v template=%v", foundDomain, foundSystem, foundMethod, foundTemplate)
	}
}

func TestOtelAttrToSlogAttr(t *testing.T) {
	tests := []struct {
		name string
		kv   attribute.KeyValue
		want slog.Attr
	}{
		{"bool", attribute.Bool("key_bool", true), slog.Bool("key_bool", true)},
		{"int64", attribute.Int64("key_int", 123), slog.Int64("key_int", 123)},
		{"float64", attribute.Float64("key_float", 1.23), slog.Float64("key_float", 1.23)},
		{"string", attribute.String("key_str", "val"), slog.String("key_str", "val")},
		{"boolslice", attribute.BoolSlice("key_bs", []bool{true, false}), slog.Any("key_bs", []bool{true, false})},
		{"int64slice", attribute.Int64Slice("key_is", []int64{1, 2}), slog.Any("key_is", []int64{1, 2})},
		{"float64slice", attribute.Float64Slice("key_fs", []float64{1.1, 2.2}), slog.Any("key_fs", []float64{1.1, 2.2})},
		{"stringslice", attribute.StringSlice("key_ss", []string{"a", "b"}), slog.Any("key_ss", []string{"a", "b"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := otelAttrToSlogAttr(tt.kv)
			if got.Key != tt.want.Key {
				t.Errorf("Key mismatch: got %v, want %v", got.Key, tt.want.Key)
			}
			if fmt.Sprintf("%v", got.Value.Any()) != fmt.Sprintf("%v", tt.want.Value.Any()) {
				t.Errorf("Value mismatch: got %v, want %v", got.Value.Any(), tt.want.Value.Any())
			}
		})
	}
}

func TestInvokeWithTracingAndLogging(t *testing.T) {
	t.Setenv("GOOGLE_SDK_GO_EXPERIMENTAL_TRACING", "true")
	TestOnlyResetIsFeatureEnabled()
	defer TestOnlyResetIsFeatureEnabled()

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	handler := &mockHandler{}
	logger := slog.New(handler)

	ctx := context.Background()

	optsTracing := []TracingOption{
		WithTracerProvider(provider),
	}
	ct := NewClientTracing(optsTracing...)

	optsLogging := []LoggingOption{
		WithLoggerProvider(logger),
	}
	cl := NewClientLogging(optsLogging...)

	callOpts := []CallOption{
		WithClientTracing(ct),
		WithClientLogging(cl),
	}

	callFunc := func(ctx context.Context, settings CallSettings) error {
		return nil
	}

	err := Invoke(ctx, callFunc, callOpts...)
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]

	if len(handler.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(handler.records))
	}
	record := handler.records[0]

	var traceID, spanID string
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "trace_id" {
			traceID = attr.Value.String()
		}
		if attr.Key == "span_id" {
			spanID = attr.Value.String()
		}
		return true
	})

	if traceID == "" || spanID == "" {
		t.Errorf("missing TraceID or SpanID in logs. traceID=%q, spanID=%q", traceID, spanID)
	}
	
	if traceID != span.SpanContext.TraceID().String() {
		t.Errorf("TraceID mismatch. Log=%q, Span=%q", traceID, span.SpanContext.TraceID().String())
	}
	
	if spanID != span.SpanContext.SpanID().String() {
		t.Errorf("SpanID mismatch. Log=%q, Span=%q", spanID, span.SpanContext.SpanID().String())
	}
}

func TestInvokePanicWithTracingAndLogging(t *testing.T) {
	t.Setenv("GOOGLE_SDK_GO_EXPERIMENTAL_TRACING", "true")
	TestOnlyResetIsFeatureEnabled()
	defer TestOnlyResetIsFeatureEnabled()

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	handler := &mockHandler{}
	logger := slog.New(handler)

	ctx := context.Background()

	optsTracing := []TracingOption{
		WithTracerProvider(provider),
	}
	ct := NewClientTracing(optsTracing...)

	optsLogging := []LoggingOption{
		WithLoggerProvider(logger),
	}
	cl := NewClientLogging(optsLogging...)

	callOpts := []CallOption{
		WithClientTracing(ct),
		WithClientLogging(cl),
	}

	callFunc := func(ctx context.Context, settings CallSettings) error {
		panic("test panic")
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic")
			}
		}()
		_ = Invoke(ctx, callFunc, callOpts...)
	}()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	// Should have recorded the panic error
	if len(span.Events) == 0 {
		t.Errorf("expected span to have an error event recorded")
	}

	if len(handler.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(handler.records))
	}
	record := handler.records[0]
	
	if record.Message != "panic: test panic" {
		t.Errorf("expected panic log message, got %v", record.Message)
	}
}
