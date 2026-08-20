package egress

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type Capability string

const (
	DNS                 Capability = "dns"
	TCP                 Capability = "tcp"
	UDP                 Capability = "udp"
	HTTP                Capability = "http"
	WebSocket           Capability = "websocket"
	WebRTC              Capability = "webrtc"
	Preconnect          Capability = "preconnect"
	SpeculativeFetch    Capability = "speculative-fetch"
	ServiceWorkerUpdate Capability = "service-worker-update"
	ExtensionNetwork    Capability = "extension-network"
	Telemetry           Capability = "telemetry"
	CrashReporting      Capability = "crash-reporting"
	SafeBrowsing        Capability = "safe-browsing"
)

var forbidden = []Capability{
	DNS,
	TCP,
	UDP,
	HTTP,
	WebSocket,
	WebRTC,
	Preconnect,
	SpeculativeFetch,
	ServiceWorkerUpdate,
	ExtensionNetwork,
	Telemetry,
	CrashReporting,
	SafeBrowsing,
}

var ErrDenied = errors.New("browser network egress denied")

// Policy is intentionally not configurable: Nomad renderer contexts deny every
// network capability. Engine adapters may expose only local verified-resource
// reads outside this type.
type Policy struct{}

func (Policy) Authorize(capability Capability) error {
	for _, denied := range forbidden {
		if capability == denied {
			return fmt.Errorf("%w: %s", ErrDenied, capability)
		}
	}
	return fmt.Errorf("%w: unknown capability %q", ErrDenied, capability)
}

func ForbiddenCapabilities() []Capability {
	return append([]Capability(nil), forbidden...)
}

// CheckRendererURL allows only non-network renderer-local schemes. A real
// Firefox/Chromium integration must call this before scheme dispatch and must
// also route all lower-level capabilities through Authorize.
func (Policy) CheckRendererURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: malformed URL", ErrDenied)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "nomad":
		if parsed.User != nil || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path == "" {
			return fmt.Errorf("%w: invalid local Nomad URL", ErrDenied)
		}
		return nil
	case "data":
		return nil
	case "about":
		if raw == "about:blank" {
			return nil
		}
	}
	return fmt.Errorf("%w: scheme %q", ErrDenied, parsed.Scheme)
}
