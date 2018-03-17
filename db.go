package main

import (
	"time"

	"github.com/satori/go.uuid"
)

type OneshotEntry struct {
	URL          string
	Redirect     bool
	Creation     time.Time
	AccessExpiry int
	AccessCount  int
}

func (e *OneshotEntry) IsExpired() bool {
	return e.AccessCount > e.AccessExpiry
}

type OneshotDB struct {
	mapping map[string]OneshotEntry
}

func (d *OneshotDB) Get(uuid string) (entry OneshotEntry, ok bool) {
	entry, ok = d.mapping[uuid]
	return
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

func (d *OneshotDB) Create(entry OneshotEntry) error {
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}

	d.mapping[id.String()] = entry

	return nil
}
