# Metrics

## The metrics endpoint

`GET /metrics` on the internal server (change with `WithMetricsRoute`) serves
Prometheus metrics in the text format. By default it serves the Prometheus
default registry, which includes:

- Go runtime metrics (`go_*`): goroutines, memory, GC, and so on,
- process metrics (`process_*`): CPU, open file descriptors, and so on,
- srvr's request metrics, described below, and
- `promhttp_metric_handler_requests_total` and
  `promhttp_metric_handler_requests_in_flight`, which count scrapes of
  `/metrics` itself.

Anything you register with the default registry also appears, for example
metrics created with `promauto`:

```go
var ordersPlaced = promauto.NewCounter(prometheus.CounterOpts{
    Name: "orders_placed_total",
    Help: "Orders placed since the process started.",
})
```

## Request metrics

Every request to the external server is recorded, including requests that
match no route:

| Metric | Type | Labels |
|---|---|---|
| `srvr_http_requests_total` | counter | `route`, `method`, `code` |
| `srvr_http_request_duration_seconds` | histogram | `route`, `method`, `code` |
| `srvr_http_requests_in_flight` | gauge | |

- `route` is the matched route's path template, such as `/users/{id}`, not
  the raw path, so the number of series stays bounded. Routes registered on a
  subrouter include the subrouter's prefix (`/api/items/{id}`). A route
  without a path matcher (for example, one that only matches a host) is
  labelled with its name if it has one (`Route.Name`), or `unnamed`.
  Requests that match no route, which the router answers with 404 or 405,
  are labelled `unmatched` (the `srvr.UnmatchedRoute` constant).
- `method` is the lower-case HTTP method (`get`, `post`, …). Non-standard
  methods are recorded as `unknown`.
- `code` is the response status code.
- The histogram uses Prometheus' default buckets, from 5ms to 10s.

Requests to the internal server (probes, scrapes, log level changes) are not
recorded.

Example queries:

```promql
# Requests per second by route
sum by (route) (rate(srvr_http_requests_total[5m]))

# Share of requests that failed with a 5xx
sum(rate(srvr_http_requests_total{code=~"5.."}[5m])) / sum(rate(srvr_http_requests_total[5m]))

# 99th percentile latency by route
histogram_quantile(0.99, sum by (route, le) (rate(srvr_http_request_duration_seconds_bucket[5m])))
```

## Using your own registry

`WithMetricsRegistry` registers srvr's request metrics with a registry you
provide and serves that registry on `/metrics`, instead of the default one:

```go
reg := prometheus.NewRegistry()
reg.MustRegister(
    collectors.NewGoCollector(),
    collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
)
server := srvr.Create(srvr.WithMetricsRegistry(reg), srvr.WithExternalRouter(router))
```

A new registry is empty, so add the Go and process collectors yourself if you
want them, as above. A separate registry is also useful in tests, which
otherwise share the global default registry.

Several `Server`s may share a registry. They then share the request metrics,
so their requests are counted together. If a registry already holds a
different metric with one of srvr's names, `Create` panics.

## Scraping

Point Prometheus at the internal port:

```yaml
scrape_configs:
  - job_name: my-service
    static_configs:
      - targets: ["my-service:8081"]
```
