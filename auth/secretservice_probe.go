package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus"
)

// secretServiceProbeTimeout bounds the pre-flight check so a stale or wedged
// session bus cannot hold up the keyring decision.
var secretServiceProbeTimeout = 2 * time.Second

const secretServiceName = "org.freedesktop.secrets"

// errSecretServiceUnavailable marks a failed pre-flight probe.
var errSecretServiceUnavailable = errors.New("secret service unavailable")

// probeSecretService checks, without starting a bus daemon, that the session bus
// socket is reachable and that org.freedesktop.secrets is owned or activatable.
// It is swapped in tests.
var probeSecretService = defaultProbeSecretService

func defaultProbeSecretService() error {
	addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if addr == "" {
		return fmt.Errorf("%w: DBUS_SESSION_BUS_ADDRESS is not set", errSecretServiceUnavailable)
	}
	ctx, cancel := context.WithTimeout(context.Background(), secretServiceProbeTimeout)
	defer cancel()

	// Cheap reachability check first: a stale address fails here immediately.
	if sock := unixSocketOf(addr); sock != "" {
		c, err := (&net.Dialer{}).DialContext(ctx, "unix", sock)
		if err != nil {
			return fmt.Errorf("%w: session bus not reachable", errSecretServiceUnavailable)
		}
		c.Close()
	}

	errc := make(chan error, 1)
	go func() { errc <- queryBus(ctx, addr) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return fmt.Errorf("%w: session bus did not respond within %s", errSecretServiceUnavailable, secretServiceProbeTimeout)
	}
}

// queryBus connects to the existing bus at addr and looks for the Secret Service.
func queryBus(ctx context.Context, addr string) error {
	conn, err := dbus.Dial(addr)
	if err != nil {
		return fmt.Errorf("%w: session bus not reachable", errSecretServiceUnavailable)
	}
	// Auth and Hello take no context: closing the connection when the deadline
	// passes is what unblocks them and releases the socket.
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-finished:
		}
	}()
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return fmt.Errorf("%w: session bus authentication failed", errSecretServiceUnavailable)
	}
	if err := conn.Hello(); err != nil {
		return fmt.Errorf("%w: session bus handshake failed", errSecretServiceUnavailable)
	}
	bus := conn.BusObject()
	var owned bool
	if err := bus.CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, secretServiceName).Store(&owned); err == nil && owned {
		return nil
	}
	var activatable []string
	if err := bus.CallWithContext(ctx, "org.freedesktop.DBus.ListActivatableNames", 0).Store(&activatable); err == nil {
		for _, n := range activatable {
			if n == secretServiceName {
				return nil
			}
		}
	}
	return fmt.Errorf("%w: no %s on the session bus", errSecretServiceUnavailable, secretServiceName)
}

// unixSocketOf extracts a dialable unix socket path from a D-Bus address, or ""
// when the address is not a plain unix one (for example tcp:).
func unixSocketOf(addr string) string {
	for _, entry := range strings.Split(addr, ";") {
		if !strings.HasPrefix(entry, "unix:") {
			continue
		}
		for _, kv := range strings.Split(strings.TrimPrefix(entry, "unix:"), ",") {
			switch {
			case strings.HasPrefix(kv, "path="):
				return strings.TrimPrefix(kv, "path=")
			case strings.HasPrefix(kv, "abstract="):
				return "@" + strings.TrimPrefix(kv, "abstract=")
			}
		}
	}
	return ""
}
