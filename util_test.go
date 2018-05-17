package main

import (
	"net"
	"net/http"
	"testing"
)

func TestRemoteAddr(t *testing.T) {
	tests := []struct {
		RemoteAddr   string
		XRealIP      string
		ConnectingIP string
		Result       string
	}{{"162.158.246.25:83789", "", "8.8.8.8", "8.8.8.8"}}

	for _, test := range tests {
		header := make(http.Header)
		header.Add("CF-Connecting-IP", test.ConnectingIP)
		header.Add("X-Real-IP", test.XRealIP)

		actual := RealRemoteIP(&http.Request{
			RemoteAddr: test.RemoteAddr,
			Header:     header,
		})

		expected := net.ParseIP(test.Result)
		if !expected.Equal(actual) {
			t.Errorf("For %#v got %s, expected %s", test, actual, expected)
		}
	}
}
