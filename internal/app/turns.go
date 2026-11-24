// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/disk"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// TurnsFile remembers on this computer which disk got the last backups, so
// the disks can be used in turn. It is in the log folder, because a backup
// runs without administrator rights. It is not signed and only decides the
// order of registered disks: without it (or with a damaged one) every disk
// is accepted, so a backup is never lost because of it.
const (
	TurnsFile    = "usb-backup.turns.json"
	maxTurnsFile = 256 << 10
)

// DiskTurn is the last backup on one disk.
type DiskTurn struct {
	// Seq orders the backups; it does not depend on the clock.
	Seq uint64 `json:"seq"`
	// Period is the label of the period.
	Period string    `json:"period"`
	Time   time.Time `json:"time"`
}

// Turns maps the ID of a disk to its last backup.
type Turns struct {
	Version int                 `json:"version"`
	Disks   map[string]DiskTurn `json:"disks"`
}

// ReadTurns reads the turns. A missing file gives no turns; so does an
// unreadable one, together with the error.
func ReadTurns(dir string, cfg *config.Config) (*Turns, error) {
	none := func() *Turns { return &Turns{Version: 1, Disks: map[string]DiskTurn{}} }
	root, err := os.OpenRoot(cfg.LogDir(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return none(), nil
	}
	if err != nil {
		return none(), err
	}
	defer func() { _ = root.Close() }()
	data, err := readLimited(root, TurnsFile, maxTurnsFile)
	if errors.Is(err, fs.ErrNotExist) {
		return none(), nil
	}
	if err != nil {
		return none(), err
	}
	t := none()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(t); err != nil {
		return none(), fmt.Errorf("%s: %w", TurnsFile, err)
	}
	if t.Disks == nil {
		t.Disks = map[string]DiskTurn{}
	}
	return t, nil
}

// Record notes a backup on disk id and drops disks that are not registered
// any more.
func (t *Turns) Record(reg *disk.Registry, id string, per week.Period, now time.Time) {
	known := map[string]DiskTurn{}
	var last uint64
	for _, d := range reg.Disks {
		if dt, ok := t.Disks[d.ID]; ok && dt.Seq < 1<<62 { // a planted number must not overflow
			known[d.ID] = dt
			last = max(last, dt.Seq)
		}
	}
	known[id] = DiskTurn{Seq: last + 1, Period: per.Label(), Time: now}
	t.Disks = known
}

// Write saves the turns.
func (t *Turns) Write(dir string, cfg *config.Config) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", TurnsFile, err)
	}
	d := cfg.LogDir(dir)
	if err := os.MkdirAll(d, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(d)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return fsx.RootWriteFileAtomic(root, TurnsFile, append(data, '\n'), 0o644)
}

// chooseDisks returns the disks that are due in period per.
func chooseDisks(deps *Deps, cfg *config.Config, reg *disk.Registry, per week.Period) (week.Turn, *Turns, error) {
	turns, err := ReadTurns(deps.ConfigDir, cfg)
	if err != nil {
		deps.Log.Warn("cannot read which disk is due, every disk is accepted", "err", err)
	}
	turn, err := turns.choose(cfg, reg, per)
	return turn, turns, err
}

func (t *Turns) choose(cfg *config.Config, reg *disk.Registry, per week.Period) (week.Turn, error) {
	slots := make([]week.Slot, len(reg.Disks))
	for i, d := range reg.Disks {
		dt := t.Disks[d.ID]
		slots[i] = week.Slot{Name: d.Name, Seq: dt.Seq, Period: dt.Period}
	}
	turn, err := week.Choose(per, cfg.Rotation, slots)
	if err != nil {
		return week.Turn{}, Errorf(KindUsage, err, "err.config", err.Error())
	}
	return turn, nil
}

// recordTurn notes the backup on c and returns the disks that are next, or
// an error when that could not be saved.
func recordTurn(deps *Deps, cfg *config.Config, reg *disk.Registry, c *Candidate, per week.Period, now time.Time) ([]string, error) {
	_, turns, err := chooseDisks(deps, cfg, reg, per)
	if err != nil {
		return nil, err
	}
	turns.Record(reg, c.Reg.ID, per, now)
	if err := turns.Write(deps.ConfigDir, cfg); err != nil {
		return nil, err
	}
	turn, err := turns.choose(cfg, reg, per)
	return turn.Next, err
}
