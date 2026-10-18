# Health Checks

srvr serves two probes on the internal server, in the form Kubernetes
expects.

## Liveness

`GET /live` (change with `WithLivenessManagement`) always responds
`200 OK` with the body `OK`. It shows the process is running and its
internal server is accepting requests. It checks nothing else, so a
dependency outage won't cause the orchestrator to restart your pods.

## Readiness

`GET /ready` (change with `WithReadinessManagement`) runs every check added
with `WithReadinessCheck`:

```go
server := srvr.Create(
    srvr.WithReadinessCheck("database", db.PingContext),
    srvr.WithReadinessCheck("cache", func(ctx context.Context) error {
        return redisClient.Ping(ctx).Err()
    }),
)
```

- If every check returns `nil`, it responds `200 OK` with the body `OK`. With
  no checks it always does.
- At the first check that returns an error, it responds
  `503 Service Unavailable` with the body
  `Readiness check '<name>' failed: <error>`, logs `Readiness check failed`
  at warn level, and skips the remaining checks.

Checks run one at a time, in no particular order, on every probe request.
Each receives the probe request's context, which is cancelled if the caller
disconnects.

## Writing checks

- **Bound the time.** srvr adds no timeout. A check that hangs makes the
  probe hang until the orchestrator's probe timeout. Wrap slow checks:

  ```go
  srvr.WithReadinessCheck("database", func(ctx context.Context) error {
      ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
      defer cancel()
      return db.PingContext(ctx)
  })
  ```

- **Keep them cheap.** Probes run every few seconds on every replica. Prefer a
  ping over a query.
- **Check what this replica needs to serve traffic**, not every downstream
  service. If a dependency that all replicas share goes down, every replica
  turns unready and the service gets no traffic at all, which may be worse
  than serving partial errors.
- **Mind the error text.** It is returned in the response body, so don't put
  secrets such as connection strings in it.

## Kubernetes

```yaml
containers:
  - name: app
    ports:
      - name: http
        containerPort: 8080
      - name: internal
        containerPort: 8081
    livenessProbe:
      httpGet: { path: /live, port: internal }
    readinessProbe:
      httpGet: { path: /ready, port: internal }
      periodSeconds: 5
      timeoutSeconds: 2
```

Expose only the `http` port in your Service.

See `examples/health` for a runnable program whose readiness changes as a
simulated database comes up.
