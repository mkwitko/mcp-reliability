// H1 / E1 harness: github.com/olgasafonova/mcp-otel-go (server middleware, default
// options) with a plain go-sdk client. The client has NO MCP instrumentation unless
// -inject is passed, so we measure the library's defaults.
//
//	go run . -role server
//	go run . -role client [-inject]
//
// The OTLP exporter honours OTEL_EXPORTER_OTLP_ENDPOINT (e.g. http://localhost:4320).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/olgasafonova/mcp-otel-go/mcpotel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

var (
	role   = flag.String("role", "server", "server or client")
	addr   = flag.String("addr", "127.0.0.1:8767", "server listen address / client target")
	inject = flag.Bool("inject", false, "client: add the 10-line sending middleware that injects traceparent into params._meta")
)

type metaCarrier map[string]any

func (c metaCarrier) Get(k string) string { v, _ := c[k].(string); return v }
func (c metaCarrier) Set(k, v string)     { c[k] = v }
func (c metaCarrier) Keys() []string {
	ks := make([]string, 0, len(c))
	for k := range c {
		ks = append(ks, k)
	}
	return ks
}

// injectMiddleware is the sketch from docs/upstream/mcp-otel-go.md (ambient span, no CLIENT span).
func injectMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if p := req.GetParams(); p != nil && !isNil(p) {
			meta := p.GetMeta()
			if meta == nil {
				meta = map[string]any{}
			}
			otel.GetTextMapPropagator().Inject(ctx, metaCarrier(meta))
			p.SetMeta(meta)
		}
		return next(ctx, method, req)
	}
}

// isNil guards go-sdk v1.8.0 typed-nil params (e.g. *InitializedParams).
func isNil(p mcp.Params) bool {
	v := reflect.ValueOf(p)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

func setupOTel(ctx context.Context, service string) (*sdktrace.TracerProvider, error) {
	exp, err := otlptracehttp.New(ctx) // endpoint from OTEL_EXPORTER_OTLP_ENDPOINT
	if err != nil {
		return nil, err
	}
	res, _ := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName(service)))
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, nil
}

type echoIn struct {
	Text string `json:"text"`
}

func runServer(ctx context.Context) error {
	tp, err := setupOTel(ctx, "mcpotel-server")
	if err != nil {
		return err
	}
	defer tp.Shutdown(context.Background())

	srv := mcp.NewServer(&mcp.Implementation{Name: "mcpotel-server", Version: "0.0.1"}, nil)
	srv.AddReceivingMiddleware(mcpotel.Middleware(mcpotel.Config{ServiceName: "mcpotel-server"}))
	mcp.AddTool(srv, &mcp.Tool{Name: "echo", Description: "echo text"},
		func(ctx context.Context, req *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: in.Text}}}, nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "fail", Description: "always fails as a tool error"},
		func(ctx context.Context, req *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "boom"}}}, nil, nil
		})

	hs := &http.Server{Addr: *addr, Handler: mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)}
	go func() { <-ctx.Done(); _ = hs.Shutdown(context.Background()) }()
	log.Printf("server listening on %s", *addr)
	if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runClient(ctx context.Context) error {
	tp, err := setupOTel(ctx, "mcpotel-client")
	if err != nil {
		return err
	}
	defer tp.Shutdown(context.Background())

	c := mcp.NewClient(&mcp.Implementation{Name: "mcpotel-client", Version: "0.0.1"}, nil)
	if *inject {
		c.AddSendingMiddleware(injectMiddleware)
	}
	ctx, root := tp.Tracer("example/mcp-client").Start(ctx, "agent step")
	defer root.End()
	sess, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://" + *addr}, nil)
	if err != nil {
		return err
	}
	defer sess.Close()
	fmt.Printf("client trace id: %s\n", root.SpanContext().TraceID())

	for _, name := range []string{"echo", "fail", "missing"} {
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: map[string]any{"text": "hello secret-token-123"}})
		switch {
		case err != nil:
			fmt.Printf("%s: rpc error: %v\n", name, err)
		case res.IsError:
			fmt.Printf("%s: tool error\n", name)
		default:
			fmt.Printf("%s: ok\n", name)
		}
	}
	return nil
}

func main() {
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var err error
	switch *role {
	case "server":
		err = runServer(ctx)
	case "client":
		err = runClient(ctx)
	default:
		err = fmt.Errorf("unknown role %q", *role)
	}
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
}
