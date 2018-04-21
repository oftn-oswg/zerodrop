package main

import (
	"errors"
	"sort"
	"strconv"
	"time"

	uuid "github.com/satori/go.uuid"
)

type ZerodropEntry struct {
	db                   *ZerodropDB
	Name                 string            // The request URI used to access this entry
	URL                  string            // The URL that this entry references
	Redirect             bool              // Indicates whether to redirect instead of proxy
	Creation             time.Time         // The time this entry was created
	AccessBlacklist      ZerodropBlacklist // Blacklist
	AccessBlacklistCount int               // Number of requests that have been caught by the blacklist
	AccessExpire         bool              // Indicates whether to expire after finite access
	AccessExpireCount    int               // The number of requests on this entry before expiry
	AccessCount          int               // The number of times this has been accessed
	AccessTrain          bool              // Whether training is active
}

// ZerodropDB represents a database connection.
// TODO: Use a persistent backend.
type ZerodropDB struct {
	mapping map[string]ZerodropEntry
}

func (d *ZerodropDB) Connect() error {
	d.mapping = map[string]ZerodropEntry{}
	return nil
}

func (d *ZerodropDB) Get(name string) (entry ZerodropEntry, ok bool) {
	entry, ok = d.mapping[name]
	return
}

// List returns a slice of all entries sorted by creation time,
// with the most recent first.
func (d *ZerodropDB) List() []ZerodropEntry {
	list := []ZerodropEntry{}

	for _, entry := range d.mapping {
		list = append(list, entry)
	}

	sort.Slice(list, func(i, j int) bool {
		a := list[i].Creation
		b := list[j].Creation
		return a.After(b)
	})

	return list
}

func (db *ZerodropDB) Create(entry *ZerodropEntry) error {
	entry.db = db

	if entry.Name == "" {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		entry.Name = id.String()
	}

	db.mapping[entry.Name] = *entry
	return nil
}

func (d *ZerodropDB) Remove(name string) {
	delete(d.mapping, name)
}

func (d *ZerodropDB) Clear() {
	d.Connect()
}

// IsExpired returns true if the entry is expired
func (e *ZerodropEntry) IsExpired() bool {
	return e.AccessExpire && (e.AccessCount >= e.AccessExpireCount)
}

func (e *ZerodropEntry) Update() error {
	if e.db != nil {
		return e.db.Create(e)
	}
	return errors.New("No link to DB")
}

// SetTraining sets the AccessTrain flag
func (e *ZerodropEntry) SetTraining(train bool) error {
	e.AccessTrain = train
	return e.Update()
}

// Access increases the access count for an entry.
func (e *ZerodropEntry) Access() error {
	e.AccessCount++
	return e.Update()
}

func (e *ZerodropEntry) String() string {
	urltype := "proxy"
	if e.Redirect {
		urltype = "redirect"
	}
	access := strconv.Itoa(e.AccessCount)
	if e.AccessExpire {
		access += "/" + strconv.Itoa(e.AccessExpireCount)
	}
	return e.Name + " {" +
		e.URL + " (" + urltype + ") " +
		access + " " + e.AccessBlacklist.String() + "}"
}
