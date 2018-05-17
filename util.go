package main

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/oftn-oswg/ipcat"
)

// ParseSocketName produces a struct suitable for net.Dial
// given a string representing a socket address to bind or connect to
func ParseSocketName(value string) (string, string) {
	value = strings.TrimSpace(value)

	// If value begins with "unix:" then we are a Unix domain socket
	if strings.Index(value, "unix:") == 0 {
		return "unix", strings.TrimSpace(value[5:])
	}

	// If value is a port number, prepend a colon
	if _, err := strconv.Atoi(value); err == nil {
		return "tcp", ":" + value
	}

	return "tcp", value
}

func getCloudflareSet() *ipcat.IntervalSet {
	ipset := ipcat.NewIntervalSet(24)
	cloudflareRanges, err := ipcat.DownloadCloudflare()
	if err != nil {
		log.Printf("Could not download Cloudflare ranges: %s", err)
		return nil
	}
	if err := ipcat.UpdateCloudflare(ipset, cloudflareRanges); err != nil {
		log.Printf("Could not update Cloudflare ranges: %s", err)
		return nil
	}
	log.Printf("Loaded %d Cloudflare records\n", ipset.Len())
	return ipset
}

var cloudflareSet = getCloudflareSet()

func peelRemoteIP(r *http.Request) net.IP {
	log.Printf("peelRemoteIP %s", r.RemoteAddr)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// We could not parse the host and port.
		// Assume this means local.
		return net.IPv6loopback
	}
	return net.ParseIP(host)
}

func peelLocalProxy(ip net.IP, r *http.Request) net.IP {
	log.Printf("peelLocalProxy %s", ip)
	if ip != nil {
		// Local proxies only!
		if ip.IsLoopback() {
			if real := r.Header.Get("X-Real-IP"); real != "" {
				headerip := net.ParseIP(real)
				if headerip != nil {
					return headerip
				}
			}
		}
	}
	return ip
}

func peelCloudflare(ip net.IP, r *http.Request) net.IP {
	log.Printf("peelCloudflare %s", ip)
	if cloudflareSet != nil {
		// Cloudflare proxy only!
		record, err := cloudflareSet.Contains(ip.String())
		if err == nil && record != nil {
			// We are being served by Cloudflare
			connectingIP := r.Header.Get("CF-Connecting-IP")
			if connectingIP != "" {
				return net.ParseIP(connectingIP)
			}
		}
	}
	return ip
}

// RealRemoteIP returns the value of the X-Real-IP header,
// or the RemoteAddr property if the header does not exist.
func RealRemoteIP(r *http.Request) net.IP {
	return peelCloudflare(peelLocalProxy(peelRemoteIP(r), r), r)
}
