package main

import (
	"errors"
	"net"
	"strings"
)

const defaultPort = "2420"

var errNoLANIP = errors.New("no non-loopback IPv4 found; pass -addr host:port")

func defaultListenAddr() (string, error) {
	ip := detectLANIPv4()
	if ip == "" {
		return "", errNoLANIP
	}
	return net.JoinHostPort(ip, defaultPort), nil
}

// advertiseAddr is the host:port printed in URLs. Unspecified and loopback
// listens are rewritten to lanIP so browsers and CLI do not use 127.0.0.1.
func advertiseAddr(listen, lanIP string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return listen
	}
	if needsAdvertiseRewrite(host) {
		if lanIP == "" {
			return listen
		}
		return net.JoinHostPort(lanIP, port)
	}
	return listen
}

func needsAdvertiseRewrite(host string) bool {
	h := strings.Trim(host, "[]")
	return h == "" || h == "0.0.0.0" || h == "::" || h == "127.0.0.1" || h == "::1" || strings.EqualFold(h, "localhost")
}

func detectLANIPv4() string {
	if ip := outboundIPv4(); ip != nil && !ip.IsLoopback() {
		return ip.String()
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var fallback net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip := ipFromAddr(a)
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.To4() == nil {
				continue
			}
			if ip.IsPrivate() {
				return ip.String()
			}
			if fallback == nil {
				fallback = ip
			}
		}
	}
	if fallback != nil {
		return fallback.String()
	}
	return ""
}

func outboundIPv4() net.IP {
	c, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil
	}
	defer c.Close()
	ua, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok || ua.IP == nil {
		return nil
	}
	return ua.IP.To4()
}

func ipFromAddr(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}
