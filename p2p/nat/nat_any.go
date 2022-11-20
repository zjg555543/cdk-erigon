package nat

import (
	"context"
	"net"
	"time"

	nat2 "github.com/libp2p/go-nat"
)

type AnyT struct {
	nat2.NAT
}

func (n AnyT) ExternalIP() (net.IP, error) { return n.GetExternalAddress() }
func (n AnyT) String() string              { return n.Type() }
func (n AnyT) AddMapping(protocol string, extport, intport int, name string, lifetime time.Duration) error {
	_, err := n.AddPortMapping(protocol, intport, name, lifetime)
	return err
}
func (n AnyT) DeleteMapping(protocol string, extport, intport int) error {
	return n.DeletePortMapping(protocol, intport)
}
func (n AnyT) SupportsMapping() bool { return true }

// Any returns a port mapper that tries to discover any supported
// mechanism on the local network.
func Any2() Interface {
	n2, err := nat2.DiscoverGateway(context.Background())
	if err != nil {
		panic(err)
	}
	return AnyT{n2}
}
