package downloadernat

import (
	"net"

	"github.com/ledgerwatch/erigon/p2p/nat"
	"github.com/ledgerwatch/log/v3"
)

func ExtIp(natif nat.Interface) (ipv4 net.IP, ipv6 net.IP) {
	switch natif.(type) {
	case nil:
		// No NAT interface, do nothing.
	case nat.ExtIP:
		// ExtIP doesn't block, set the IP right away.
		ip, _ := natif.ExternalIP()
		if ip != nil {
			if ip.To4() != nil {
				return ip, nil
			} else {
				return nil, ip
			}
		}
		log.Info("[torrent] Public IP", "ip", ip)

	default:
		// Ask the router about the IP. This takes a while and blocks startup,
		// do it in the background.
		if ip, err := natif.ExternalIP(); err == nil {
			if ip != nil {
				if ip.To4() != nil {
					return ip, nil
				} else {
					return nil, ip
				}
			}
			log.Info("[torrent] Public IP", "ip", ip)
		}
	}
	return nil, nil
}
