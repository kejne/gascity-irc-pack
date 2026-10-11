# gascity-irc-pack

A standard-library Go adapter that forwards IRC `PRIVMSG` messages to a Gas
City `extmsg` endpoint. Runtime values are environment-specific; the
repository contains no server, account, or credential-specific configuration.

## Configuration

Copy `config.example.env` to an untracked file such as `config.local.env` and
fill in the values for your IRC and Gas City environments. Never commit
credentials.

- `IRC_SERVER`, `IRC_PORT`, and `IRC_CHANNEL` select the IRC service.
- `IRC_NICK`, `IRC_SASL_USERNAME`, and `IRC_PASSWORD` configure identity and
  optional SASL PLAIN authentication.
- `GC_API_BASE_URL`, `GC_CITY_NAME`, `GC_PROVIDER`, `GC_ACCOUNT_ID`, and
  `GC_TARGET` select the Gas City destination.
- Set `GC_REGISTER_ADAPTER=false` when the adapter is already registered.

## Run

```sh
set -a
. ./config.local.env
set +a
go run ./cmd/app
```

The bridge connects over TLS, joins the configured channel, retries dropped
connections, and forwards the first non-empty matching message. Delivery uses
Gas City's wire-compatible envelope at:

```
POST /v0/city/<GC_CITY_NAME>/extmsg/inbound
```

## Development

Install the pinned toolchain and hooks, then run the canonical checks:

```sh
mise install
hk install
hk check
```

Run the full suite, including security checks, with:

```sh
mise exec -- hk run pre-push
```

The focused tests use `net.Pipe` and `httptest`; they do not contact IRC or Gas
City.
