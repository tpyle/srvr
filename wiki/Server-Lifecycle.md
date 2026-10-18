# Server Lifecycle

```go
server := srvr.Create(options...)  // configure; nothing listens yet
server.Start()                     // bind the ports and serve in the background
server.Wait()                      // block until the servers have stopped
server.Stop()                      // stop gracefully
```

## Create

`Create` builds the servers and registers routes and middleware on the
routers. It does not open any ports.

## Start

`Start` binds the internal and external ports, starts serving each in its own
goroutine, and returns. When it returns `nil`, both servers are accepting
connections.

If either port can't be bound, for example because it is already in use,
`Start` closes anything it opened and returns the error. Neither server is
running, and you can call `Start` again later.

```go
if err := server.Start(); err != nil {
    // e.g. "srvr: starting external server: listen tcp :8080: bind: address already in use"
    return err
}
```

`Start` returns `srvr.ErrServerStarted` if the server is already running, and
`srvr.ErrServerStopped` if it has been stopped. A stopped `Server` cannot be
restarted; call `Create` for a new one.

### Listening addresses

`InternalAddr()` and `ExternalAddr()` return the addresses the servers are
listening on, or `nil` before `Start` (or for a server that isn't
configured). With port 0 the operating system picks a free port, and these
methods tell you which one. That's handy in tests:

```go
server := srvr.Create(srvr.WithInternalPort(0), srvr.WithExternalPort(0), srvr.WithExternalRouter(router))
if err := server.Start(); err != nil {
    t.Fatal(err)
}
defer server.Stop()
resp, err := http.Get("http://" + server.ExternalAddr().String() + "/hello")
```

## Stop and Shutdown

`Shutdown(ctx)` shuts both servers down at the same time, gracefully:

1. They stop accepting new connections and close idle ones.
2. Requests already in progress run to completion.
3. If `ctx` ends first, the remaining connections are closed immediately and
   `Shutdown` returns the context's error, such as
   `context.DeadlineExceeded`.

Both servers are always shut down; if both fail, their errors are joined.

`Stop()` is `Shutdown` with a timeout taken from `WithShutdownTimeout`
(default 10 seconds, or the `srvr.shutdownTimeout` [Viper key](Viper.md)).

Long-lived connections such as WebSockets are not waited for; close them
yourself, for example with `http.Server.RegisterOnShutdown` on your own
servers or by watching a context.

## Wait and Run

`Wait` blocks until both servers have stopped serving and, if `Stop` or
`Shutdown` was called, until it has finished draining. After `Wait` returns,
it is safe to exit.

`Run` is `Start` followed by `Wait`. If `Start` fails, `Run` logs
`Failed to start server` and returns immediately. Use `Start` and `Wait`
directly if you need the error. Something else must call `Stop` for `Run` to
return, so it suits programs where another goroutine handles shutdown.

## Handling signals

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

server := srvr.Create(options...)
if err := server.Start(); err != nil {
    return err
}

<-ctx.Done()
if err := server.Stop(); err != nil {
    return err
}
server.Wait()
```

## Kubernetes

When a pod is deleted, Kubernetes starts shutting its containers down and, at
the same time, marks its endpoints as terminating
([Termination of Pods](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination)).
kube-proxy, ingress controllers and cloud load balancers pick that change up a
little later, so new requests can arrive after `SIGTERM`. Because `Shutdown`
stops accepting connections immediately, those requests would be refused.

Kubernetes does not tell you to delay shutdown, but it is the common fix: wait a
few seconds before stopping. The simplest way is a `preStop` hook, which runs
before the container receives `SIGTERM`:

```yaml
spec:
  terminationGracePeriodSeconds: 30
  containers:
    - name: api
      lifecycle:
        preStop:
          sleep:
            seconds: 5
```

The `sleep` action needs a recent Kubernetes version. On older clusters use
`exec: {command: ["sleep", "5"]}` (the image needs a `sleep` binary) or sleep in
your program after the signal:

```go
<-ctx.Done()
time.Sleep(5 * time.Second) // let endpoint changes propagate
if err := server.Stop(); err != nil {
    return err
}
```

The grace period covers the `preStop` hook as well as shutdown. Keep the delay
plus the shutdown timeout below `terminationGracePeriodSeconds`, or the pod is
killed before draining finishes.
