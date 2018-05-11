package main

import "testing"

func TestIntersection(t *testing.T) {
	tests := []struct {
		Mi     Geofence
		Tu     Geofence
		Result SetIntersection
	}{
		{Geofence{36.1699, -115.1398, 1000.0}, Geofence{36.1699, -115.1398, 10.0}, IsSuperset},
		{Geofence{36.1699, -115.1398, 10.0}, Geofence{36.1699, -115.1398, 1000.0}, IsSubset},
		{Geofence{36.1699, -115.1398, 10.0}, Geofence{37.7749, -122.4194, 1000.0}, IsDisjoint},
		{Geofence{36.1699, -115.1398, 10.0}, Geofence{36.1699, -115.1398, 10.0}, IsSubset | IsSuperset},
		{Geofence{36.1699, -115.13983, 100.0}, Geofence{36.1699, -115.1398, 100.0}, 0},
	}

	for _, test := range tests {
		got := test.Mi.Intersection(&test.Tu)
		want := test.Result
		if want != got {
			t.Errorf("With %s and %s: expected intersection code %b, got %b",
				BlacklistRule{Geofence: &test.Mi}, BlacklistRule{Geofence: &test.Tu}, want, got)
		}
	}
}
