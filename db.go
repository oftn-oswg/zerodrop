package main

import (
	"sort"
	"time"

	"github.com/satori/go.uuid"
)

type ZerodropEntry struct {
	UUID         string
	URL          string
	Redirect     bool
	Creation     time.Time
	AccessExpiry int
	AccessCount  int
}

func (e *ZerodropEntry) IsExpired() bool {
	return e.AccessCount >= e.AccessExpiry
}

type ZerodropDB struct {
	mapping map[string]ZerodropEntry
}

func (d *ZerodropDB) Connect() error {
	d.mapping = map[string]ZerodropEntry{}
	return nil
}

func (d *ZerodropDB) Get(uuid string) (entry ZerodropEntry, ok bool) {
	entry, ok = d.mapping[uuid]
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

func (d *ZerodropDB) Access(uuid string) (entry ZerodropEntry, ok bool) {
	entry, ok = d.Get(uuid)

	if ok {
		updatedEntry := entry
		updatedEntry.AccessCount++
		d.mapping[uuid] = updatedEntry
	}

	return
}

func (d *ZerodropDB) Create(entry *ZerodropEntry) error {
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}

	entry.UUID = id.String()
	d.mapping[entry.UUID] = *entry

	return nil
}

func (d *ZerodropDB) Remove(uuid string) {
	delete(d.mapping, uuid)
}

func (d *ZerodropDB) Clear() {
	d.Connect()
}
