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
		"err.otherVault": {EN: "this vault does not belong to this installation (its signing key differs)", NL: "deze vault hoort niet bij deze installatie (de ondertekeningssleutel verschilt)"},
		"cfg.changed":    {EN: "%s changed from %s to %s.", NL: "%s gewijzigd van %s naar %s."},
		"cfg.unsigned":   {EN: "Note: no keys yet, so the settings are not signed. \"keys init\" signs them.", NL: "Let op: er zijn nog geen sleutels, dus de instellingen zijn niet ondertekend. \"keys init\" ondertekent ze."},
	})
}

func init() {
	add(map[string]entry{
		"cfg.sourceAdded":   {EN: "Source %s (%s) added.", NL: "Bronmap %s (%s) toegevoegd."},
		"cfg.sourceRemoved": {EN: "Source %s removed. Its backups stay on the disks.", NL: "Bronmap %s verwijderd. De back-ups blijven op de schijven staan."},
		"cfg.exported":      {EN: "Settings written to %s (without keys or vault).", NL: "Instellingen geschreven naar %s (zonder sleutels of vault)."},
		"cfg.imported":      {EN: "Settings imported from %s and signed.", NL: "Instellingen geïmporteerd uit %s en ondertekend."},
	})
}
