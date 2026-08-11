package gax

import (
	"context"
	"github.com/googleapis/gax-go/v2/callctx"
	"log/slog"
	"testing"
)

func TestInvokeWithLogging_ContextCanceled(t *testing.T) {
	t.Setenv("GOOGLE_SDK_GO_EXPERIMENTAL_TRACING", "true")
	TestOnlyResetIsFeatureEnabled()
	defer TestOnlyResetIsFeatureEnabled()

	handler := &mockHandler{}
	logger := slog.New(handler)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = callctx.WithTelemetryContext(ctx, "rpc_method", "my.test.Method", "url_template", "/v1/foo?query=123")

	opts := []LoggingOption{
		WithLoggerProvider(logger),
	}
	cl := NewClientLogging(opts...)
	callOpts := []CallOption{WithClientLogging(cl)}

	callFunc := func(ctx context.Context, settings CallSettings) error {
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}

	err := Invoke(ctx, callFunc, callOpts...)
	if err == nil {
		t.Fatalf("Invoke() expected error")
	}

	if len(handler.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(handler.records))
	}
	record := handler.records[0]

	if record.Message != "context canceled" {
		t.Errorf("expected msg 'context canceled', got %q", record.Message)
	}
	if record.Level != slog.LevelError {
		t.Errorf("expected level error, got %v", record.Level)
	}
	var hasErrorType bool
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "error.type" && attr.Value.String() == "CLIENT_CANCELLED" {
			hasErrorType = true
		}
		return true
	})
	if !hasErrorType {
		t.Errorf("missing error.type=CLIENT_CANCELLED")
	}
}
