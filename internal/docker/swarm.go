package docker

import (
	"fmt"
	"net"
	"strings"
)

// SwarmInit initializes a Docker swarm, auto-selecting the best advertise address.
func SwarmInit(exec *Executor) error {
	_, err := exec.Run("swarm", "init")
	if err != nil {
		// If init fails due to multiple addresses, try to pick one automatically
		if strings.Contains(err.Error(), "advertise address") {
			addr, addrErr := selectAdvertiseAddr()
			if addrErr != nil {
				return fmt.Errorf("initialize swarm: %w (auto-detect failed: %v)", err, addrErr)
			}
			_, err = exec.Run("swarm", "init", "--advertise-addr", addr)
			if err != nil {
				return fmt.Errorf("initialize swarm with %s: %w", addr, err)
			}
			return nil
		}
		return fmt.Errorf("initialize swarm: %w", err)
	}
	return nil
}

// SwarmInitWithAddr initializes a Docker swarm with a specific advertise address.
func SwarmInitWithAddr(exec *Executor, addr string) error {
	_, err := exec.Run("swarm", "init", "--advertise-addr", addr)
	if err != nil {
		return fmt.Errorf("initialize swarm with %s: %w", addr, err)
	}
	return nil
}

// ListAdvertiseAddrs returns non-loopback IPv4 addresses suitable for swarm advertise-addr.
func ListAdvertiseAddrs() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	var addrs []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ifAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range ifAddrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
				addrs = append(addrs, ipNet.IP.String())
			}
		}
	}
	return addrs, nil
}

// selectAdvertiseAddr picks the best advertise address automatically.
// Prefers private network addresses (192.168.x.x, 10.x.x.x, 172.16-31.x.x).
func selectAdvertiseAddr() (string, error) {
	addrs, err := ListAdvertiseAddrs()
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("no suitable network addresses found")
	}

	// Prefer private addresses
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && ip.IsPrivate() {
			return addr, nil
		}
	}

	return addrs[0], nil
}
