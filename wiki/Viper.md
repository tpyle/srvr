# Configuring with Viper

srvr can read its ports, timeouts and logging switches from a
[Viper](https://github.com/spf13/viper) instance, so they can come from
config files, environment variables, flags, or anything else Viper supports.

Viper is only consulted when you pass `WithViper` or `WithViperPrefix`.
Values set on the global Viper instance are ignored otherwise.

```go
v := viper.New()
v.SetConfigFile("config.yaml")
if err := v.ReadInConfig(); err != nil {
    return err
}

server := srvr.Create(
    srvr.WithExternalRouter(router),
    srvr.WithViper(v),
)
```

## Keys

`WithViper(v)` reads the keys below. `WithViperPrefix(v, prefix)` reads the
same keys with `prefix` in front, e.g. `api.srvr.externalPort` for the prefix
`api.`. Include the trailing dot in the prefix.

| Key | Type | Default |
|---|---|---|
| `srvr.externalPort` | int | 8080 |
| `srvr.internalPort` | int | 8081 |
| `srvr.readTimeout` | duration | 5 |
| `srvr.writeTimeout` | duration | 10 |
| `srvr.idleTimeout` | duration | 120 |
| `srvr.readHeaderTimeout` | duration | 2 |
| `srvr.shutdownTimeout` | duration | 10 |
| `srvr.maxHeaderBytes` | int | 1048576 |
| `srvr.doLogInternalRouter` | bool | true |
| `srvr.doLogExternalRouter` | bool | true |

Viper keys are case-insensitive, so `srvr.externalport` works too.

Durations are either a number of seconds, which may be fractional (`15`,
`0.5`), or a Go duration string (`"1m30s"`, `"500ms"`). Both forms work from
config files and environment variables. An invalid duration is ignored and the
previous value is kept. `Create` logs it at warn level (`Ignoring invalid
duration in configuration`) with the logger from `WithLogger`, wherever that
option appears in the list.

Routers, readiness checks, endpoint paths, and the logger can only be set
with options.

## Precedence

srvr reads Viper when the `WithViper`/`WithViperPrefix` option is applied:

1. Options **before** it act as defaults. srvr registers their values with
   `v.SetDefault` (durations as seconds), so a key that is set in Viper
   overrides them.
2. Options **after** it override Viper.

```go
srvr.Create(
    srvr.WithExternalPort(3000), // used unless srvr.externalPort is set
    srvr.WithViper(v),
    srvr.WithReadTimeout(time.Minute), // always used
)
```

Because of the `SetDefault` calls, `v.Get("srvr.externalPort")` returns the
effective value afterwards, even if your config didn't set it.

## Config file

```yaml
srvr:
  externalPort: 8080
  internalPort: 8081
  readTimeout: 15
  writeTimeout: 30
  doLogInternalRouter: false
```

With a prefix, several services can share one file:

```yaml
api:
  srvr:
    externalPort: 8080
    internalPort: 8081
worker:
  srvr:
    externalPort: 8090
    internalPort: 8091
```

```go
apiServer := srvr.Create(srvr.WithViperPrefix(v, "api."), srvr.WithExternalRouter(apiRouter))
workerServer := srvr.Create(srvr.WithViperPrefix(v, "worker."))
```

## Environment variables

Viper's `AutomaticEnv` maps keys to environment variable names by upper-casing
them. Keys contain dots, which are awkward in variable names, so add a key
replacer:

```go
v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
v.AutomaticEnv()
```

Then `SRVR_EXTERNALPORT=3000` sets `srvr.externalPort`, and with the prefix
`api.`, `API_SRVR_EXTERNALPORT=3000` sets `api.srvr.externalPort`.

If you also call `v.SetEnvPrefix("MYAPP")`, Viper prepends `MYAPP_`:
`MYAPP_SRVR_EXTERNALPORT`.

See `examples/viper` for a runnable program that combines a YAML file with
environment overrides.
