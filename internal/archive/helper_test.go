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

package archive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// WriteFileTo is a test helper: it writes dest+".part" through a root on
// the folder of dest.
func WriteFileTo(t testing.TB, src, dest string, opts Options) (*Result, error) {
	return WriteFileToCtx(t, context.Background(), src, dest, opts)
}

// WriteFileToCtx is WriteFileTo with a context.
func WriteFileToCtx(t testing.TB, ctx context.Context, src, dest string, opts Options) (*Result, error) {
	t.Helper()
	root, err := os.OpenRoot(filepath.Dir(dest))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	return WriteFile(ctx, src, root, filepath.Base(dest), opts)
}
