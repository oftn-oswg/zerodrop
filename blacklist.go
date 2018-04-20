package main

import (
	"net"
	"regexp"
	"strconv"
	"strings"
)

type ZerodropBlacklistItem struct {
	Negation bool
	All      bool
	Network  *net.IPNet
	IP       net.IP
	Hostname string
	Regexp   *regexp.Regexp
}

func (i ZerodropBlacklistItem) String() (value string) {
	if i.Negation {
		value += "!"
	}

	if i.All {
		value += "*"
		return
	}

	if i.Network != nil {
		value += i.Network.String()
		return
	}

	if i.IP != nil {
		value += i.IP.String()
		return
	}

	if i.Hostname != "" {
		value += i.Hostname
		return
	}

	if i.Regexp != nil {
		value += "~"
		value += i.Regexp.String()
		return
	}

	return
}

type ZerodropBlacklist struct {
	List []*ZerodropBlacklistItem
}

func (b ZerodropBlacklist) String() string {
	items := make([]string, len(b.List)+1)
	switch l := len(b.List); l {
	case 0:
		items[0] = "# Empty blacklist"
	case 1:
		items[0] = "# Blacklist with 1 item"
	default:
		items[0] = "# Blacklist with " + strconv.Itoa(l) + " items"
	}
	for index, item := range b.List {
		items[index+1] = item.String()
	}
	return strings.Join(items, "\n")
}

func ParseBlacklist(text string) ZerodropBlacklist {
	lines := strings.Split(text, "\n")
	blacklist := ZerodropBlacklist{List: []*ZerodropBlacklistItem{}}

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
			blacklist.Add(item)
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
			blacklist.Add(item)
			continue
		}

		// If an IP address is given, parse as unique IP
		if ip := net.ParseIP(line); ip != nil {
			item.IP = ip
			blacklist.Add(item)
			continue
		}

		// Otherwise, treat the pattern as a hostname.
		item.Hostname = strings.ToLower(line)
		blacklist.Add(item)
	}

	return blacklist
}

func (b *ZerodropBlacklist) Add(item *ZerodropBlacklistItem) {
	b.List = append(b.List, item)
}

func (b *ZerodropBlacklist) Allow(ip net.IP) bool {
	allow := true

	for _, item := range b.List {
		match := false

		if item.All {
			// Wildcard
			match = true

		} else if item.Network != nil {
			// IP Network
			match = item.Network.Contains(ip)

		} else if item.IP != nil {
			// IP Address
			match = item.IP.Equal(ip)

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
