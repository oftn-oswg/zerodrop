package main

import (
	"sort"
	"time"

	"github.com/satori/go.uuid"
)

type OneshotEntry struct {
	UUID         string
	URL          string
	Redirect     bool
	Creation     time.Time
	AccessExpiry int
	AccessCount  int
}

func (e *OneshotEntry) IsExpired() bool {
	return e.AccessCount >= e.AccessExpiry
}

type OneshotDB struct {
	mapping map[string]OneshotEntry
}

func (d *OneshotDB) Connect() error {
	d.mapping = map[string]OneshotEntry{}
	return nil
}

func (d *OneshotDB) Get(uuid string) (entry OneshotEntry, ok bool) {
	entry, ok = d.mapping[uuid]
	return
}

// List returns a slice of all entries sorted by creation time,
// with the most recent first.
func (d *OneshotDB) List() []OneshotEntry {
	list := []OneshotEntry{}

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

func (d *OneshotDB) Access(uuid string) (entry OneshotEntry, ok bool) {
	entry, ok = d.Get(uuid)

	if ok {
		updatedEntry := entry
		updatedEntry.AccessCount++
		d.mapping[uuid] = updatedEntry
	}

	return
}

func (d *OneshotDB) Create(entry *OneshotEntry) error {
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}

	entry.UUID = id.String()
	d.mapping[entry.UUID] = *entry

	return nil
}
