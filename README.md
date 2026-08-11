Google API Extensions for Go
============================

[![GoDoc](https://godoc.org/github.com/googleapis/gax-go?status.svg)](https://godoc.org/github.com/googleapis/gax-go)

Google API Extensions for Go (gax-go) is a set of modules which aids the
development of APIs for clients and servers based on `gRPC` and Google API
conventions.

To install the API extensions, use:

```
go get -u github.com/googleapis/gax-go/v2
```

**Note:** Application code will rarely need to use this library directly,
but the code generated automatically from API definition files can use it
to simplify code generation and to provide more convenient and idiomatic API surface.

Go Versions
===========
This library requires Go 1.6 or above.

License
=======
BSD - please see [LICENSE](https://github.com/googleapis/gax-go/blob/main/LICENSE)
for more information.

OpenTelemetry Observability (Experimental)
==========================================

Google API generated clients now experimentally support observability using OpenTelemetry. You can selectively configure distributed tracing (with `trace.TracerProvider`) and structured logging (with `slog.Logger`) when instantiating these clients. 

Because clients now accept these inputs implicitly through standard configuration structures wrapped by `google.golang.org/api/option`, you can provide them directly to the client constructor.

Here is an example demonstrating how to set them using OpenTelemetry:

```go
import (
	"context"
	"log/slog"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	
	// Import your generated client package here (e.g., cloud.google.com/go/storage)
)

func initializeClient(ctx context.Context, tp trace.TracerProvider, logger *slog.Logger) error {
	// Configure the client selectively with a TracerProvider and a structured Logger.
	clientOpts := []option.ClientOption{
		// Wrap standard OpenTelemetry TracerProvider configuration via gRPC DialOption
		option.WithGRPCDialOption(grpc.WithStatsHandler(otelgrpc.NewClientHandler(otelgrpc.WithTracerProvider(tp)))),
		// (For REST clients, you would use option.WithHTTPClient with otelhttp instead)
		
		// Configure structured logging explicitly using the standard slog Logger
		option.WithLogger(logger),
	}
	
	// Example: instantiating a client 
	// client, err := generatedclient.NewClient(ctx, clientOpts...)
	// return err
	
	return nil
}
```
