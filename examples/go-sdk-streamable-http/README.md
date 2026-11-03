# Go SDK scenario harness

One binary, hand-rolled OTel middleware, flags that inject one defect each. Built on go-sdk v1.8.0. The directory keeps its name although it also speaks stdio.

```sh
go run . -role server [-break-error-type] [-leak-arguments] [-drop-method-attr] [-capture f.jsonl]
go run . -role client [-break-propagation] [-bad-traceparent]
```

`-transport http` (default): Streamable HTTP on `-addr`.

`-transport stdio`: run only the client. It spawns this same binary with `-role server -transport stdio` through `mcp.CommandTransport`; the server uses `mcp.StdioTransport`, so stdout is the protocol channel and logs go to stderr. Server flags go in one string:

```sh
go run . -role client -transport stdio -break-propagation -server-args "-capture out.wire.jsonl -leak-arguments"
```

Both roles export spans and the `mcp.{client,server}.operation.duration` histogram (unit `s`, 1 s export interval) to the OTLP/HTTP collector on 4318, and flush both on exit.

Fixtures: `make fixtures` (Docker). `ONLY=healthy-stdio examples/run-scenarios.sh` runs one scenario.
