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
		"status.period":       {EN: "Period %s: %s is due.", NL: "Periode %s: %s is aan de beurt."},
		"status.noDisks":      {EN: "No backup disk connected.", NL: "Geen back-upschijf aangesloten."},
		"status.disk":         {EN: "%s (%s): health %s, %s free of %s, backups: %d, last backup %s%s", NL: "%s (%s): gezondheid %s, %s vrij van %s, back-ups: %d, laatste back-up %s%s"},
		"status.never":        {EN: "never", NL: "nooit"},
		"status.dueMark":      {EN: " (due now)", NL: " (nu aan de beurt)"},
		"status.needsUpdate":  {EN: "  This disk must be updated by the operator (keys rewrap).", NL: "  Deze schijf moet door de beheerder worden bijgewerkt (keys rewrap)."},
		"status.bgNever":      {EN: "Warning: the break-glass key has never been tested. Test it with \"verify --identity <break-glass file>\".", NL: "Let op: de break-glass-sleutel is nog nooit getest. Test hem met \"verify --identity <break-glass-bestand>\"."},
		"status.bgTested":     {EN: "Break-glass key last tested: %s.", NL: "Break-glass-sleutel laatst getest: %s."},
		"status.notEncrypted": {EN: "Backups are not encrypted.", NL: "De back-ups zijn niet versleuteld."},
	})
}
