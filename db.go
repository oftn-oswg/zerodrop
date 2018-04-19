package main

import (
	"sort"
	"time"

	uuid "github.com/satori/go.uuid"
)

type ZerodropEntry struct {
	Name              string            // The request URI used to access this entry
	URL               string            // The URL that this entry references
	Redirect          bool              // Indicates whether to redirect instead of proxy
	Creation          time.Time         // The time this entry was created
	AccessBlacklist   ZerodropBlacklist // Blacklist
	AccessExpire      bool              // Indicates whether to expire after finite access
	AccessExpireCount int               // The number of requests on this entry before expiry
	AccessCount       int               // The number of times this has been accessed
	AccessTrain       bool              // Whether training is active
}

func (e *ZerodropEntry) IsExpired() bool {
	return e.AccessExpire && (e.AccessCount >= e.AccessExpireCount)
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

func (d *ZerodropDB) SetTraining(name string, train bool) bool {
	updatedEntry, ok := d.mapping[name]
	if !ok {
		return false
	}

	updatedEntry.AccessTrain = train
	d.mapping[name] = updatedEntry
	return true
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

func (d *ZerodropDB) Access(name string) (entry ZerodropEntry, ok bool) {
	entry, ok = d.Get(name)

	if ok {
		updatedEntry := entry
		updatedEntry.AccessCount++
		d.mapping[name] = updatedEntry
	}

	return
}

func (d *ZerodropDB) Create(entry *ZerodropEntry) error {
	if entry.Name == "" {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		entry.Name = id.String()
	}

	d.mapping[entry.Name] = *entry

	return nil
}

func (d *ZerodropDB) Remove(name string) {
	delete(d.mapping, name)
}

func (d *ZerodropDB) Clear() {
	d.Connect()
}
