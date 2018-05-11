package main

import (
	"github.com/kellydunn/golang-geo"
)

// Geofence represents a point on the Earth with an accuracy radius in meters.
type Geofence struct {
	Latitude, Longitude, Radius float64
}

// SetIntersection is a description of the relationship between two sets.
type SetIntersection uint

const (
	// IsDisjoint means that the two sets have no common elements.
	IsDisjoint SetIntersection = 1 << iota

	// IsSubset means the first set is a subset of the second.
	IsSubset

	// IsSuperset means the second set is a subset of the first.
	IsSuperset
)

// Intersection describes the relationship between two geofences
func (mi *Geofence) Intersection(tu *Geofence) (i SetIntersection) {
	miPoint := geo.NewPoint(mi.Latitude, mi.Longitude)
	tuPoint := geo.NewPoint(tu.Latitude, tu.Longitude)
	distance := miPoint.GreatCircleDistance(tuPoint) * 1000

	radiusSum := mi.Radius + tu.Radius
	radiusDiff := mi.Radius - tu.Radius

	if distance-radiusSum > 0 {
		i = IsDisjoint
		return
	}

	if -distance+radiusDiff >= 0 {
		i |= IsSuperset
	}

	if -distance-radiusDiff >= 0 {
		i |= IsSubset
	}

	return
}
