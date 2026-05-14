package tunnel

import (
	"fmt"
	"net"

	"github.com/BrunoSilvaFreire/tunneld/internal/constants"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"google.golang.org/protobuf/proto"
)

// PortMapping records a single resolved local forward: the configured port
// (which may be 0 for dynamic allocation) and the actual port the OS assigned.
type PortMapping struct {
	LocalAddress   string
	ConfiguredPort int32
	ActualPort     int32
	RemotePort     int32 // remote/target port, for display
}

// ResolvePortsInProto clones spec, replaces every port=0 forward with an
// OS-assigned port (via a temporary net.Listen), and returns the modified
// clone plus the resolved mappings. Non-zero ports are included in mappings
// unchanged (configured == actual).
func ResolvePortsInProto(spec *pb.TunnelSpec) (*pb.TunnelSpec, []PortMapping, error) {
	clone := proto.Clone(spec).(*pb.TunnelSpec)
	var mappings []PortMapping

	if ssh := clone.GetSsh(); ssh != nil {
		for _, f := range ssh.LocalForwards {
			addr := f.ListenAddress
			if addr == "" {
				addr = constants.DefaultListenAddress
			}
			configured := f.ListenPort
			actual := configured
			if actual == 0 {
				p, err := allocatePort(addr)
				if err != nil {
					return nil, nil, fmt.Errorf("port alloc for ssh forward: %w", err)
				}
				actual = p
				f.ListenPort = actual
			}
			mappings = append(mappings, PortMapping{
				LocalAddress:   addr,
				ConfiguredPort: configured,
				ActualPort:     actual,
				RemotePort:     f.TargetPort,
			})
		}
	}

	if kc := clone.GetKubectl(); kc != nil {
		for _, f := range kc.Forwards {
			addr := f.LocalAddress
			if addr == "" {
				addr = constants.DefaultListenAddress
			}
			configured := f.LocalPort
			actual := configured
			if actual == 0 {
				p, err := allocatePort(addr)
				if err != nil {
					return nil, nil, fmt.Errorf("port alloc for kubectl forward: %w", err)
				}
				actual = p
				f.LocalPort = actual
			}
			mappings = append(mappings, PortMapping{
				LocalAddress:   addr,
				ConfiguredPort: configured,
				ActualPort:     actual,
				RemotePort:     f.RemotePort,
			})
		}
	}

	return clone, mappings, nil
}

// LocalPortKeys returns "addr:port" strings for all non-zero local ports claimed
// by a spec. Port=0 forwards are skipped because they use dynamic allocation and
// cannot conflict at configuration time.
func LocalPortKeys(spec *pb.TunnelSpec) []string {
	var keys []string
	if ssh := spec.GetSsh(); ssh != nil {
		for _, f := range ssh.LocalForwards {
			if f.ListenPort == 0 {
				continue
			}
			addr := f.ListenAddress
			if addr == "" {
				addr = constants.DefaultListenAddress
			}
			keys = append(keys, net.JoinHostPort(addr, fmt.Sprintf("%d", f.ListenPort)))
		}
	}
	if kc := spec.GetKubectl(); kc != nil {
		for _, f := range kc.Forwards {
			if f.LocalPort == 0 {
				continue
			}
			addr := f.LocalAddress
			if addr == "" {
				addr = constants.DefaultListenAddress
			}
			keys = append(keys, net.JoinHostPort(addr, fmt.Sprintf("%d", f.LocalPort)))
		}
	}
	return keys
}

// allocatePort binds addr:0, captures the OS-assigned port, and immediately
// closes the listener. There is a small TOCTOU window between close and when
// the subprocess binds; this is acceptable for development-use dynamic tunnels.
func allocatePort(addr string) (int32, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(addr, "0"))
	if err != nil {
		return 0, err
	}
	port := int32(ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()
	return port, nil
}
