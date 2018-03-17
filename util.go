package main

import (
	"strconv"
	"strings"
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
