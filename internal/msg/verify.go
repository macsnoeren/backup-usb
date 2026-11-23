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
		"err.needKey":       {EN: "this backup is encrypted: give --password-stdin, --identity or enter the vault password", NL: "deze back-up is versleuteld: geef --password-stdin of --identity, of voer het vault-wachtwoord in"},
		"err.noDiskAny":     {EN: "no registered backup disk connected", NL: "geen geregistreerde back-upschijf aangesloten"},
		"err.noBackups":     {EN: "no backups found on the disk", NL: "geen back-ups gevonden op de schijf"},
		"err.noSuchPeriod":  {EN: "no backup for %s on the disk", NL: "geen back-up van %s op de schijf"},
		"err.decrypt":       {EN: "decrypting %s failed (wrong key or damaged file)", NL: "ontsleutelen van %s is mislukt (verkeerde sleutel of beschadigd bestand)"},
		"err.identity":      {EN: "key file: %s", NL: "sleutelbestand: %s"},
		"verify.full":       {EN: "OK, %d files unpacked and checked", NL: "OK, %d bestanden uitgepakt en gecontroleerd"},
		"verify.hash":       {EN: "OK, encrypted file checked against its SHA-256 (no key given, contents not unpacked)", NL: "OK, versleuteld bestand gecontroleerd tegen de SHA-256 (geen sleutel, inhoud niet uitgepakt)"},
		"verify.failed":     {EN: "FAILED: %s", NL: "MISLUKT: %s"},
		"verify.breakglass": {EN: "The break-glass key works.", NL: "De break-glass-sleutel werkt."},
	})
}

func init() {
	add(map[string]entry{
		"err.noArchive":    {EN: "no archive for tag %q in %s", NL: "geen archief voor tag %q in %s"},
		"list.noIndex":     {EN: "Index missing or damaged: the archive itself was read (this takes longer).", NL: "Index ontbreekt of is beschadigd: het archief zelf is gelezen (dit duurt langer)."},
		"list.periodsHead": {EN: "Period     Disk      Folder     Sources", NL: "Periode    Schijf    Map        Bronnen"},
	})
}

func init() {
	add(map[string]entry{
		"err.restoreTag":   {EN: "restore needs --tag", NL: "restore heeft --tag nodig"},
		"err.restoreDest":  {EN: "restore needs --to <folder> or --original", NL: "restore heeft --to <map> of --original nodig"},
		"err.restoreWrite": {EN: "cannot write %s", NL: "kan %s niet schrijven"},
		"err.restoreHash":  {EN: "%s: the restored file does not match its SHA-256; it was not kept", NL: "%s: het teruggezette bestand klopt niet met de SHA-256; het is niet bewaard"},
		"err.notInBackup":  {EN: "%s is not in this backup", NL: "%s staat niet in deze back-up"},
		"restore.done":     {EN: "Restored %d files (%s) from %s %s %s to %s.", NL: "%d bestanden (%s) teruggezet uit %s %s %s naar %s."},
		"restore.renamed":  {EN: "  %s already existed, restored as %s", NL: "  %s bestond al, teruggezet als %s"},
		"restore.skipped":  {EN: "  skipped %s (%s)", NL: "  overgeslagen: %s (%s)"},
	})
}
