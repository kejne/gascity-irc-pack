# gascity-irc-pack

A minimal IRC-to-Gas-City extmsg tracer bullet. It connects to Smedijan over
TLS, joins one configured channel, and forwards the first non-empty `PRIVMSG`
to one Gas City target. The process exits after successful delivery; dropped
IRC connections are retried until delivery or shutdown.

## Live exercise

Prerequisites: a running Gas City API and an agent/session that can receive the
configured target. The API must be reachable at `GC_API_BASE_URL`.

1. Copy `config.example.env` to an untracked file such as `config.local.env`.
2. Fill in the Smedijan nickname, channel, and SASL password. The Halloy file
   at `/home/kejne/snap/halloy/common/.config/halloy/config.toml` contains the
   local server and SASL settings; copy credentials manually, never commit or
   print them.
3. Set `GC_CITY_NAME` and `GC_TARGET` (for example, `mayor`). `GC_TARGET` is
   the configured Gas City recipient; it is carried as extmsg's
   `explicit_target`.
4. Run:

```sh
set -a
. ./config.local.env
set +a
go run ./cmd/app
```

The bridge registers the `irc`/`smedijan` adapter with Gas City, connects to
`irc.smedijan.se:6697` using TLS 1.2+, negotiates SASL PLAIN when a password is
set, joins `IRC_CHANNEL`, and posts to:

```
POST /v0/city/<GC_CITY_NAME>/extmsg/inbound
```

The request uses Gas City's wire-compatible envelope (`message.conversation`,
`message.actor`, `message.text`, `message.explicit_target`, and a dedup key).
No IRC message text or credentials are logged by the bridge.

If the adapter was registered previously, set `GC_REGISTER_ADAPTER=false`.
For a bound conversation instead of an explicit target, bind the same provider,
account, and channel with `gc extmsg bind`, then choose the corresponding
recipient configuration.

## Development

The implementation is standard-library-only Go. Run the canonical checks:

```sh
mise install
hk check
```

The focused tests use `net.Pipe` and `httptest`; they do not contact IRC or
Gas City.
