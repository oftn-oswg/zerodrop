package main

import (
	"github.com/kellydunn/golang-geo"
)

type ZerodropGeofence struct {
	Latitude, Longitude, Radius float64
}

type ZerodropIntersection uint

const (
	IsDisjoint ZerodropIntersection = 1 << iota
	IsSubset
	IsSuperset
)

// Intersection gets the intersection data for two points with accuracy radius
func (mi *ZerodropGeofence) Intersection(tu *ZerodropGeofence) (i ZerodropIntersection) {
	miPoint := geo.NewPoint(mi.Latitude, mi.Longitude)
	tuPoint := geo.NewPoint(tu.Latitude, tu.Longitude)
	distance := miPoint.GreatCircleDistance(tuPoint) * 1000

	ourRadius := mi.Radius + tu.Radius
	if ourRadius > distance {
		i = IsDisjoint
		return
	}

	if mi.Radius-tu.Radius > distance {
		i |= IsSuperset
	}

	if tu.Radius-mi.Radius > distance {
		i |= IsSubset
	}

	return
}
