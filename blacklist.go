package main

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/oftn-oswg/ipcat"
	"github.com/oschwald/geoip2-golang"
)

// BlacklistContext is a structure used to contain the external data
// used to categorize IP addresses needed for specific rules, like
// the geolocation database used for geofencing or the ipcat database.
type BlacklistContext struct {
	GeoDB     *geoip2.Reader
	Databases map[string]*ipcat.IntervalSet
}

// BlacklistRule is a structure that represents a rule or comment as part
// of a blacklist.
type BlacklistRule struct {
	Comment  string
	Negation bool
	All      bool
	Network  *net.IPNet
	IP       net.IP
	Hostname string
	Regexp   string
	Geofence *Geofence
	Database string
}

func (i BlacklistRule) String() (value string) {
	if i.Negation {
		value += "!"
	}

	if i.All {
		value += "*"
		if i.Comment != "" {
			value += " # " + i.Comment
		}
		return
	}

	if i.Network != nil {
		value += i.Network.String()
		if i.Comment != "" {
			value += " # " + i.Comment
		}
		return
	}

	if i.IP != nil {
		value += i.IP.String()
		if i.Comment != "" {
			value += " # " + i.Comment
		}
		return
	}

	if i.Hostname != "" {
		value += i.Hostname
		if i.Comment != "" {
			value += " # " + i.Comment
		}
		return
	}

	if i.Regexp != "" {
		value += "~"
		value += i.Regexp
		if i.Comment != "" {
			value += " # " + i.Comment
		}
		return
	}

	if i.Geofence != nil {
		value += "@ " +
			strconv.FormatFloat(i.Geofence.Latitude, 'f', -1, 64) + ", " +
			strconv.FormatFloat(i.Geofence.Longitude, 'f', -1, 64) + " (" +
			strconv.FormatFloat(i.Geofence.Radius, 'f', -1, 64) + "m)"
		if i.Comment != "" {
			value += " # " + i.Comment
		}
		return
	}

	if i.Database != "" {
		value += "db " + i.Database
		if i.Comment != "" {
			value += " # " + i.Comment
		}
		return
	}

	if i.Comment != "" {
		value += "# " + i.Comment
	}

	return
}

// Blacklist is a list of BlacklistRules
type Blacklist struct {
	List []*BlacklistRule
}

func (b Blacklist) String() string {
	itemCount := 0

	// Stringify items
	items := make([]string, len(b.List)+1)
	for index, item := range b.List {
		if item.All || item.Network != nil || item.Geofence != nil ||
			item.Database != "" || item.Hostname != "" || item.IP != nil ||
			item.Regexp != "" {
			itemCount++
		}
		items[index+1] = item.String()
	}

	// Blacklist comment header
	switch itemCount {
	case 0:
		items[0] = "# Empty blacklist"
	case 1:
		items[0] = "# Blacklist with 1 item"
	default:
		items[0] = "# Blacklist with " + strconv.Itoa(itemCount) + " items"
	}

	return strings.Join(items, "\n")
}

var geofenceRegexp = regexp.MustCompile(`^([-+]?[0-9]*\.?[0-9]+)[^-+0-9]+([-+]?[0-9]*\.?[0-9]+)(?:[^0-9]+([0-9]*\.?[0-9]+)([A-Za-z]*)[^0-9]*)?$`)
var geofenceUnits = map[string]float64{
	"":   1.0,
	"m":  1.0,
	"km": 1000.0,
	"mi": 1609.0,
	"ft": 1609.0 / 5280.0,
}

// ParseBlacklist parses a text blacklist and returns a Blacklist object.
func ParseBlacklist(text string, dbconfig map[string]string) Blacklist {
	lines := strings.Split(text, "\n")
	blacklist := Blacklist{List: []*BlacklistRule{}}

	for _, line := range lines {
		item := &BlacklistRule{}

		// A line with # serves as a comment.
		if commentStart := strings.IndexByte(line, '#'); commentStart >= 0 {
			item.Comment = strings.TrimSpace(line[commentStart+1:])
			line = line[:commentStart]
		}

		// A blank line matches no files,
		// so it can serve as a separator for readability.
		line = strings.TrimSpace(line)
		if line == "" {
			if item.Comment != "" {
				blacklist.Add(item)
			}
			continue
		}

		// An optional prefix "!" which negates the pattern;
		// any matching address/host excluded by a previous pattern
		// will become included again.
		if line[0] == '!' {
			item.Negation = true
			line = strings.TrimSpace(line[1:])
		}

		// A line with only "*" matches everything,
		// allowing the creation of a whitelist.
		if line == "*" {
			item.All = true
			blacklist.Add(item)
			continue
		}

		// Database query match
		if line[:3] == "db " {
			db := strings.ToLower(strings.TrimSpace(line[3:]))
			if _, ok := dbconfig[db]; !ok {
				item.Comment = fmt.Sprintf("Error: %s: No database specified named %q", line, db)
				blacklist.Add(item)
				continue
			}
			item.Database = db
			blacklist.Add(item)
			continue
		}

		switch line[0] {
		case '@':
			// An optional prefix "@" indicates a geofencing target.
			var lat, lng, radius float64 = 0, 0, 25

			line = strings.TrimSpace(line[1:])
			matches := geofenceRegexp.FindStringSubmatch(line)

			if len(matches) == 5 {
				var err error

				latString, lngString, radiusString, units :=
					matches[1], matches[2], matches[3], strings.ToLower(matches[4])

				// Parse latitude
				if lat, err = strconv.ParseFloat(latString, 64); err != nil {
					item.Comment = fmt.Sprintf(
						"Error: %s: could not parse latitude: %s",
						line, err.Error())
					blacklist.Add(item)
					continue
				}

				// Parse longitude
				if lng, err = strconv.ParseFloat(lngString, 64); err != nil {
					item.Comment = fmt.Sprintf(
						"Error: %s: could not parse longitude: %s",
						line, err.Error())
					blacklist.Add(item)
					continue
				}

				// Parse optional radius
				if radiusString != "" {
					if radius, err = strconv.ParseFloat(radiusString, 64); err != nil {
						item.Comment = fmt.Sprintf(
							"Error: %s: could not parse radius: %s",
							line, err.Error())
						blacklist.Add(item)
						continue
					}
				}

				// Parse units
				factor, ok := geofenceUnits[units]
				if !ok {
					item.Comment = fmt.Sprintf(
						"Error: %s: invalid radial units: %s",
						line, strconv.Quote(units))
					blacklist.Add(item)
					continue
				}
				radius *= factor

			} else {
				item.Comment = fmt.Sprintf(
					"Error: %s: invalid format: must be <lng>, <lng> (<radius><unit>)?",
					line)
				blacklist.Add(item)
				continue
			}

			item.Geofence = &Geofence{
				Latitude:  lat,
				Longitude: lng,
				Radius:    radius,
			}
			blacklist.Add(item)
			continue

		case '~':
			// An optional prefix "~" indicates a hostname regular expression match.
			line = strings.TrimSpace(line[1:])
			_, err := regexp.Compile(line)
			if err != nil {
				item.Comment = fmt.Sprintf(
					"Error: %s: malformed regular expression: %s",
					line, err.Error())
				blacklist.Add(item)
				continue
			}

			item.Regexp = line
			blacklist.Add(item)
			continue
		}

		// If a CIDR notation is given, then parse that as an IP network.
		_, network, err := net.ParseCIDR(line)
		if err == nil {
			item.Network = network
			blacklist.Add(item)
			continue
		}

		// If an IP address is given, parse as unique IP.
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

// Add appends a BlacklistRule to the Blacklist.
func (b *Blacklist) Add(item *BlacklistRule) {
	b.List = append(b.List, item)
}

// Allow decides whether the Blacklist permits the selected IP address.
func (b *Blacklist) Allow(ctx *BlacklistContext, ip net.IP) bool {
	allow := true

	user := (*Geofence)(nil)

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

		} else if item.Regexp != "" {
			// Regular Expression
			regex, err := regexp.Compile(item.Regexp)
			if err != nil {
				log.Printf("Error compiling regular expression: %s", err)
			}

			names, err := net.LookupAddr(ip.String())
			if err != nil {
				for _, name := range names {
					name = strings.ToLower(name)
					if regex.Match([]byte(name)) {
						match = true
						break
					}
				}
			}
		} else if item.Geofence != nil {
			if ctx.GeoDB == nil {
				log.Println("Denying access by geofence rule error: no database provided")
				return false
			}

			if user == nil {
				record, err := ctx.GeoDB.City(ip)
				if err != nil {
					log.Printf("Denying access by geofence rule error: %s", err.Error())
					return false
				}
				user = &Geofence{
					Latitude:  record.Location.Latitude,
					Longitude: record.Location.Longitude,
					Radius:    float64(record.Location.AccuracyRadius) * 1000.0, // Convert km to m
				}
			}

			bounds := item.Geofence
			boundsIntersect := bounds.Intersection(user)
			if item.Negation {
				// Whitelist if user is completely contained within bounds
				match = boundsIntersect&IsSuperset != 0
			} else {
				// Blacklist if user intersects at all with bounds
				match = !(boundsIntersect&IsDisjoint != 0)
			}
		} else if item.Database != "" {
			db, ok := ctx.Databases[item.Database]
			if !ok {
				log.Printf("Denying access by db rule error: database %q not provided", item.Database)
				return false
			}

			interval, err := db.Contains(ip.String())
			if err != nil {
				log.Printf("Denying access by db rule error: %s", err.Error())
				return false
			}

			match = interval != nil
		}

		// TODO: Allow early termination based on negation flags
		if match {
			allow = item.Negation
		}
	}

	return allow
}
