// Scenario harness: an instrumented MCP client and server built on the official
// Go SDK, talking over Streamable HTTP, exporting spans via OTLP/HTTP.
//
// The OpenTelemetry middleware here is deliberately minimal and hand-rolled so
// the scenario has no dependency on any third-party MCP instrumentation. Flags
// introduce controlled defects that mcp-reliability doctor must detect.
//
//	go run . -role server [-break-error-type] [-leak-arguments] [-drop-method-attr]
//	go run . -role client [-break-propagation]
//
// With -transport stdio the client spawns this binary as the server (extra
// server flags go in -server-args); stdout is then the protocol channel.
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
	"os/exec"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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
	transport        = flag.String("transport", "http", "http (Streamable HTTP) or stdio")
	serverArgs       = flag.String("server-args", "", "client, stdio only: extra flags for the spawned server, space separated")
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
// (err != nil) and tool execution errors (CallToolResult.IsError). It returns
// the outcome attributes so the duration metric can carry the same ones.
func recordOutcome(span trace.Span, res mcp.Result, err error) (out []attribute.KeyValue) {
	defer func() { span.SetAttributes(out...) }()
	var werr *jsonrpc.Error
	switch {
	case errors.As(err, &werr):
		code := strconv.FormatInt(werr.Code, 10)
		out = append(out, attribute.String("rpc.response.status_code", code))
		// semconv: -32700/-32600/-32601/-32602/-32002 are caller errors and
		// SHOULD NOT be considered failures of this span.
		if callerError[code] {
			return
		}
		if !*breakErrorType {
			out = append(out, attribute.String("error.type", code))
			span.SetStatus(codes.Error, werr.Message)
		}
	case err != nil:
		if !*breakErrorType {
			out = append(out, attribute.String("error.type", "_OTHER"))
			span.SetStatus(codes.Error, err.Error())
		}
	default:
		if r, ok := res.(*mcp.CallToolResult); ok && r.IsError && !*breakErrorType {
			out = append(out, attribute.String("error.type", "tool_error"))
			span.SetStatus(codes.Error, "tool returned isError")
		}
	}
	return
}

// opDuration is mcp.{client,server}.operation.duration (Histogram, unit s).
var opDuration metric.Float64Histogram

// observe records one operation. The metric carries the same attributes as the
// span except gen_ai.tool.call.arguments, which never belongs on a metric.
func observe(ctx context.Context, start time.Time, base, outcome []attribute.KeyValue) {
	attrs := make([]attribute.KeyValue, 0, len(base)+len(outcome))
	for _, kv := range append(base, outcome...) {
		if kv.Key != "gen_ai.tool.call.arguments" {
			attrs = append(attrs, kv)
		}
	}
	opDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
}

func clientMiddleware(tracer trace.Tracer) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			start, base := time.Now(), baseAttrs(method, req)
			ctx, span := tracer.Start(ctx, spanName(method, req),
				trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(base...))
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
			observe(ctx, start, base, recordOutcome(span, res, err))
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
			start, base := time.Now(), baseAttrs(method, req)
			ctx, span := tracer.Start(ctx, spanName(method, req),
				trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(base...))
			defer span.End()
			res, err := next(ctx, method, req)
			observe(ctx, start, base, recordOutcome(span, res, err))
			return res, err
		}
	}
}

// setupOTel installs trace and metric providers and returns a shutdown func
// that flushes both.
func setupOTel(ctx context.Context, service string) (*sdktrace.TracerProvider, func(), error) {
	exp, err := otlptracehttp.New(ctx, otlptracehttp.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	mexp, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	res, _ := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName(service)))
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(mexp, sdkmetric.WithInterval(time.Second))))
	name := "mcp." + *role + ".operation.duration"
	opDuration, err = mp.Meter("example/mcp-"+*role).Float64Histogram(name, metric.WithUnit("s"),
		metric.WithDescription("Duration of an MCP request or notification."),
		metric.WithExplicitBucketBoundaries(0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30, 60, 120, 300))
	if err != nil {
		return nil, nil, err
	}
	// The global propagator is a no-op by default; without this line nothing is
	// injected or extracted. This is a common real-world misconfiguration.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, func() { _ = tp.Shutdown(context.Background()); _ = mp.Shutdown(context.Background()) }, nil
}

type echoIn struct {
	Text string `json:"text"`
}

func runServer(ctx context.Context) error {
	tp, shutdown, err := setupOTel(ctx, "example-mcp-server")
	if err != nil {
		return err
	}
	defer shutdown()
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

	if *transport == "stdio" {
		// stdout belongs to the protocol; log already writes to stderr.
		return srv.Run(ctx, &mcp.StdioTransport{})
	}
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
	tp, shutdown, err := setupOTel(ctx, "example-mcp-client")
	if err != nil {
		return err
	}
	defer shutdown()
	tracer := tp.Tracer("example/mcp-client")

	c := mcp.NewClient(&mcp.Implementation{Name: "example-client", Version: "0.0.1"}, nil)
	c.AddSendingMiddleware(clientMiddleware(tracer))
	var t mcp.Transport = &mcp.StreamableClientTransport{Endpoint: "http://" + *addr}
	if *transport == "stdio" {
		cmd := exec.Command(os.Args[0], append([]string{"-role", "server", "-transport", "stdio"}, strings.Fields(*serverArgs)...)...)
		cmd.Stderr = os.Stderr
		t = &mcp.CommandTransport{Command: cmd}
	}
	sess, err := c.Connect(ctx, t, nil)
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
