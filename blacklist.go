package main

import (
	"net"
	"regexp"
	"strings"
)

type ZerodropBlacklistItem struct {
	Negation bool
	All      bool
	Network  *net.IPNet
	Hostname string
	Regexp   *regexp.Regexp
}

type ZerodropBlacklist []*ZerodropBlacklistItem

func ParseBlacklist(text string) ZerodropBlacklist {
	lines := strings.Split(text, "\n")
	blacklist := ZerodropBlacklist{}

	for _, line := range lines {
		item := &ZerodropBlacklistItem{}

		// A line with # serves as a comment.
		if commentStart := strings.IndexByte(text, '#'); commentStart >= 0 {
			line = line[:commentStart]
		}

		// A blank line matches no files,
		// so it can serve as a separator for readability.
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// An optional prefix "!" which negates the pattern;
		// any matching address/host excluded by a previous pattern
		// will become included again.
		if line[0] == '!' {
			item.Negation = true
			line = line[1:]
		}

		// A line with only "*" matches everything
		if line == "*" {
			item.All = true
			blacklist = append(blacklist, item)
			continue
		}

		// An optional prefix "~" indicates a hostname regular expression match
		if line[0] == '~' {
			line = strings.TrimSpace(line[1:])
			reg, err := regexp.Compile(line)
			if err != nil {
				item.Regexp = reg
			} else {
				continue
			}
		}

		// If a CIDR notation is given, then parse that as an IP network
		_, network, err := net.ParseCIDR(line)
		if err == nil {
			item.Network = network
			blacklist = append(blacklist, item)
			continue
		}

		// If an IP address is given, parse as unique IP
		if ip := net.ParseIP(line); ip != nil {
			bits := len(ip) * 8
			mask := net.CIDRMask(bits, bits)
			if mask != nil {
				item.Network = &net.IPNet{
					IP:   ip,
					Mask: mask,
				}
				blacklist = append(blacklist, item)
				continue
			}
		}

		// Otherwise, treat the pattern as a hostname.
		item.Hostname = strings.ToLower(line)
		blacklist = append(blacklist, item)
	}

	return blacklist
}

func (b ZerodropBlacklist) Allow(ip net.IP) bool {
	allow := true

	for _, item := range b {
		match := false

		if item.All {
			// Wildcard
			match = true

		} else if item.Network != nil {
			// IP Network
			match = item.Network.Contains(ip)

		} else if item.Hostname != "" {
			// Hostname
			addrs, err := net.LookupIP(item.Hostname)
			if err != nil {
				for _, addr := range addrs {
					if addr.Equal(ip) {
						match = true
						break
					}
				}
			}

			names, err := net.LookupAddr(ip.String())
			if err != nil {
				for _, name := range names {
					name = strings.ToLower(name)
					if name == item.Hostname {
						match = true
						break
					}
				}
			}

		} else if item.Regexp != nil {
			// Regular Expression
			names, err := net.LookupAddr(ip.String())
			if err != nil {
				for _, name := range names {
					name = strings.ToLower(name)
					if item.Regexp.Match([]byte(name)) {
						match = true
						break
					}
				}
			}

		}

		if match {
			allow = item.Negation
		}
	}

	return allow
}
