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

package msg

func init() {
	add(map[string]entry{
		"err.install":    {EN: "the task %q could not be registered", NL: "de taak %q kon niet worden geregistreerd"},
		"err.uninstall":  {EN: "the task %q could not be removed", NL: "de taak %q kon niet worden verwijderd"},
		"task.installed": {EN: "Task %q registered.", NL: "Taak %q geregistreerd."},
		"task.removed":   {EN: "Task %q removed.", NL: "Taak %q verwijderd."},
	})
}
