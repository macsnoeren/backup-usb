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

//go:build !windows && !linux

package volume

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("volume detection is not supported on this platform")

type stubFinder struct{}

// NewPlatformFinder returns the finder for this platform.
func NewPlatformFinder() Finder { return stubFinder{} }

func (stubFinder) Volumes(context.Context) ([]Volume, error) { return nil, errUnsupported }
func (stubFinder) Space(context.Context, Volume) (uint64, uint64, error) {
	return 0, 0, errUnsupported
}
func (stubFinder) Health(context.Context, Volume) Health { return Unknown }
func (stubFinder) Mount(_ context.Context, v Volume) (Volume, func() error, error) {
	return v, func() error { return nil }, errUnsupported
}
func (stubFinder) Eject(context.Context, Volume) error { return errUnsupported }
