// Scenario harness: an instrumented MCP client and server built on the official
// Go SDK, talking over Streamable HTTP, exporting spans via OTLP/HTTP.
//
// The OpenTelemetry middleware here is deliberately minimal and hand-rolled so
// the scenario has no dependency on any third-party MCP instrumentation. Flags
// introduce controlled defects that mcp-reliability doctor must detect.
//
//	go run . -role server [-break-error-type] [-leak-arguments] [-drop-method-attr]
//	go run . -role client [-break-propagation]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	role             = flag.String("role", "server", "server or client")
	addr             = flag.String("addr", "127.0.0.1:8765", "server listen address / client target")
	breakPropagation = flag.Bool("break-propagation", false, "client: do not inject traceparent into params._meta")
	breakErrorType   = flag.Bool("break-error-type", false, "server: omit error.type and leave status Unset on failed tool calls")
	leakArguments    = flag.Bool("leak-arguments", false, "server: record raw tool arguments as gen_ai.tool.call.arguments")
	dropMethodAttr   = flag.Bool("drop-method-attr", false, "server: omit mcp.method.name")
	badTraceparent   = flag.Bool("bad-traceparent", false, "client: send a malformed _meta.traceparent")
	capturePath      = flag.String("capture", "", "server: append received {method,_meta} JSON lines to this file")
)

var captureMu sync.Mutex

// capture appends one wire record per received message. The doctor's -wire
// flag reads this format to check SEP-414 _meta formats.
func capture(method string, meta map[string]any) {
	if *capturePath == "" {
		return
	}
	captureMu.Lock()
	defer captureMu.Unlock()
	f, err := os.OpenFile(*capturePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("capture: %v", err)
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(map[string]any{"method": method, "_meta": meta})
}

// metaCarrier adapts params._meta to the OTel TextMapCarrier interface.
type metaCarrier map[string]any

func (c metaCarrier) Get(k string) string {
	if v, ok := c[k].(string); ok {
		return v
	}
	return ""
}
func (c metaCarrier) Set(k, v string) { c[k] = v }
func (c metaCarrier) Keys() []string {
	ks := make([]string, 0, len(c))
	for k := range c {
		ks = append(ks, k)
	}
	return ks
}

// toolCall returns the tool name and raw arguments for tools/call requests.
// The client sees *CallToolParams; the server sees *CallToolParamsRaw.
func toolCall(req mcp.Request) (name string, args any, ok bool) {
	switch p := req.GetParams().(type) {
	case *mcp.CallToolParams:
		return p.Name, p.Arguments, true
	case *mcp.CallToolParamsRaw:
		return p.Name, p.Arguments, true
	}
	return "", nil, false
}

// params returns the request params, treating a typed-nil pointer as absent.
// go-sdk v1.8.0 hands middleware a typed-nil *InitializedParams for
// notifications sent without params, and calling GetMeta on it panics.
func params(req mcp.Request) mcp.Params {
	p := req.GetParams()
	if p == nil {
		return nil
	}
	if v := reflect.ValueOf(p); v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	return p
}

func spanName(method string, req mcp.Request) string {
	if name, _, ok := toolCall(req); ok && method == "tools/call" {
		return method + " " + name
	}
	return method
}

func baseAttrs(method string, req mcp.Request) []attribute.KeyValue {
	attrs := []attribute.KeyValue{}
	if !*dropMethodAttr || *role == "client" {
		attrs = append(attrs, attribute.String("mcp.method.name", method))
	}
	if name, args, ok := toolCall(req); ok && method == "tools/call" {
		attrs = append(attrs,
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.name", name))
		if *leakArguments && *role == "server" {
			b, _ := json.Marshal(args)
			attrs = append(attrs, attribute.String("gen_ai.tool.call.arguments", string(b)))
		}
	}
	return attrs
}

var callerError = map[string]bool{"-32700": true, "-32600": true, "-32601": true, "-32602": true, "-32002": true}

// recordOutcome maps the two MCP failure shapes onto the span: JSON-RPC errors
// (err != nil) and tool execution errors (CallToolResult.IsError).
func recordOutcome(span trace.Span, res mcp.Result, err error) {
	var werr *jsonrpc.Error
	switch {
	case errors.As(err, &werr):
		code := strconv.FormatInt(werr.Code, 10)
		span.SetAttributes(attribute.String("rpc.response.status_code", code))
		// semconv: -32700/-32600/-32601/-32602/-32002 are caller errors and
		// SHOULD NOT be considered failures of this span.
		if callerError[code] {
			return
		}
		if !*breakErrorType {
			span.SetAttributes(attribute.String("error.type", strconv.FormatInt(werr.Code, 10)))
			span.SetStatus(codes.Error, werr.Message)
		}
	case err != nil:
		if !*breakErrorType {
			span.SetAttributes(attribute.String("error.type", "_OTHER"))
			span.SetStatus(codes.Error, err.Error())
		}
	default:
		if r, ok := res.(*mcp.CallToolResult); ok && r.IsError && !*breakErrorType {
			span.SetAttributes(attribute.String("error.type", "tool_error"))
			span.SetStatus(codes.Error, "tool returned isError")
		}
	}
}

func clientMiddleware(tracer trace.Tracer) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			ctx, span := tracer.Start(ctx, spanName(method, req),
				trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(baseAttrs(method, req)...))
			defer span.End()
			if p := params(req); p != nil && !*breakPropagation {
				m := p.GetMeta()
				if m == nil {
					m = map[string]any{}
				}
				otel.GetTextMapPropagator().Inject(ctx, metaCarrier(m))
				if *badTraceparent {
					m["traceparent"] = "not-a-w3c-traceparent"
				}
				p.SetMeta(m)
			}
			res, err := next(ctx, method, req)
			recordOutcome(span, res, err)
			return res, err
		}
	}
}

func serverMiddleware(tracer trace.Tracer) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if p := params(req); p != nil {
				capture(method, p.GetMeta())
				if p.GetMeta() != nil {
					ctx = otel.GetTextMapPropagator().Extract(ctx, metaCarrier(p.GetMeta()))
				}
			}
			ctx, span := tracer.Start(ctx, spanName(method, req),
				trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(baseAttrs(method, req)...))
			defer span.End()
			res, err := next(ctx, method, req)
			recordOutcome(span, res, err)
			return res, err
		}
	}
}

func setupOTel(ctx context.Context, service string) (*sdktrace.TracerProvider, error) {
	exp, err := otlptracehttp.New(ctx, otlptracehttp.WithInsecure())
	if err != nil {
		return nil, err
	}
	res, _ := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName(service)))
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	// The global propagator is a no-op by default; without this line nothing is
	// injected or extracted. This is a common real-world misconfiguration.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, nil
}

type echoIn struct {
	Text string `json:"text"`
}

func runServer(ctx context.Context) error {
	tp, err := setupOTel(ctx, "example-mcp-server")
	if err != nil {
		return err
	}
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("example/mcp-server")

	srv := mcp.NewServer(&mcp.Implementation{Name: "example-server", Version: "0.0.1"}, nil)
	srv.AddReceivingMiddleware(serverMiddleware(tracer))
	mcp.AddTool(srv, &mcp.Tool{Name: "echo", Description: "echo text"},
		func(ctx context.Context, req *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: in.Text}}}, nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "fail", Description: "always fails as a tool error"},
		func(ctx context.Context, req *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "boom"}}}, nil, nil
		})

	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	hs := &http.Server{Addr: *addr, Handler: h}
	go func() {
		<-ctx.Done()
		_ = hs.Shutdown(context.Background())
	}()
	log.Printf("server listening on %s", *addr)
	if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runClient(ctx context.Context) error {
	tp, err := setupOTel(ctx, "example-mcp-client")
	if err != nil {
		return err
	}
	defer tp.Shutdown(context.Background())
	tracer := tp.Tracer("example/mcp-client")

	c := mcp.NewClient(&mcp.Implementation{Name: "example-client", Version: "0.0.1"}, nil)
	c.AddSendingMiddleware(clientMiddleware(tracer))
	sess, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://" + *addr}, nil)
	if err != nil {
		return err
	}
	defer sess.Close()

	// One root span per logical agent step so client spans are not themselves roots.
	ctx, root := tracer.Start(ctx, "agent step")
	defer root.End()

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
	time.Sleep(200 * time.Millisecond) // let the batcher flush before Shutdown
}
