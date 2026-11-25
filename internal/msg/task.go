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
		"task.on":        {EN: "A backup starts when a registered disk is plugged in.", NL: "Een back-up start als een geregistreerde schijf wordt ingestoken."},
		"task.off":       {EN: "Nothing starts when a disk is plugged in.", NL: "Er start niets als een schijf wordt ingestoken."},
		"task.loginOn":   {EN: "The window starts, hidden, when a user logs in.", NL: "Het venster start, verborgen, als een gebruiker inlogt."},
		"err.trayOff":    {EN: "switch on trayIcon first: without it the window cannot run hidden", NL: "zet eerst trayIcon aan: zonder die instelling kan het venster niet verborgen draaien"},
		"err.noWindow":   {EN: "usb-backup.exe is not next to usb-backup-cli; starting at login is for Windows only", NL: "usb-backup.exe staat niet naast usb-backup-cli; starten bij inloggen kan alleen op Windows"},
		"err.install":    {EN: "the task %q could not be registered", NL: "de taak %q kon niet worden geregistreerd"},
		"err.uninstall":  {EN: "the task %q could not be removed", NL: "de taak %q kon niet worden verwijderd"},
		"task.installed": {EN: "Task %q registered.", NL: "Taak %q geregistreerd."},
		"task.removed":   {EN: "Task %q removed.", NL: "Taak %q verwijderd."},
	})
}
