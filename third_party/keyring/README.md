# Local fork of github.com/99designs/keyring v1.2.2

Wired in with a `replace` in the root `go.mod`. MIT licensed, see the upstream
project for the original licence.

Why: upstream's `secretservice.go` and `kwallet.go` call `dbus.SessionBus()` from
`init()`. That runs before any owlctl code, so with `DBUS_SESSION_BUS_ADDRESS`
unset godbus autolaunches a `dbus-daemon` (one leaked per run), and with an
unreachable address every command (even `--version`) hangs.

Changes from upstream:
- `secretservice.go`: the Secret Service backend is always registered on Linux and
  connects lazily when opened (owlctl probes the bus first, see
  `auth/secretservice_probe.go`). No `init()`-time bus connection.
- Dropped the kwallet, keyctl and pass backends (owlctl never selects them; kwallet
  had the same init-time `SessionBus()`), the upstream tests and the CLI.
