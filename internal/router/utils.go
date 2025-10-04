package router

import "net"

func checkIpWhitelist(clientAddr net.Addr, ipWhitelist []string) bool {
	host, _, err := net.SplitHostPort(clientAddr.String())
	if err != nil {
		return false
	}

	clientIP := net.ParseIP(host)
	if clientIP == nil {
		return false
	}

	for _, entry := range ipWhitelist {
		ip := net.ParseIP(entry)
		if ip != nil {
			if ip.Equal(clientIP) {
				return true
			}
			continue
		}

		ips, err := net.LookupIP(entry)
		if err != nil {
			continue
		}
		for _, domainIP := range ips {
			if domainIP.Equal(clientIP) {
				return true
			}
		}
	}

	return false
}
