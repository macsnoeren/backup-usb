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
		"err.config":          {EN: "invalid settings: %s", NL: "ongeldige instellingen: %s"},
		"err.listDisks":       {EN: "the disks cannot be listed", NL: "de schijven kunnen niet worden opgesomd"},
		"err.wrongDisk":       {EN: "%s is for %s, but %s is connected", NL: "%s is voor %s, maar %s is aangesloten"},
		"err.unknownDisk":     {EN: "unknown disk %q: this disk is not registered (ID file, signature or serial numbers do not match); nothing was written", NL: "onbekende schijf %q: deze schijf is niet geregistreerd (ID-bestand, handtekening of serienummers kloppen niet); er is niets geschreven"},
		"err.noDisk":          {EN: "no backup disk connected; insert %s", NL: "geen back-upschijf aangesloten; plaats %s"},
		"err.duplicate":       {EN: "skipped: a backup was made less than %d minutes ago", NL: "overgeslagen: minder dan %d minuten geleden is al een back-up gemaakt"},
		"err.recent":          {EN: "skipped: this folder was backed up less than %d hours ago", NL: "overgeslagen: deze map is minder dan %d uur geleden al gemaakt"},
		"err.noSources":       {EN: "no source folders configured; use \"config add-source\"", NL: "geen bronmappen ingesteld; gebruik \"config add-source\""},
		"err.unknownTag":      {EN: "no source folder with tag %q", NL: "geen bronmap met tag %q"},
		"err.registry":        {EN: "the registration list cannot be read", NL: "de registratielijst kan niet worden gelezen"},
		"err.sourceMissing":   {EN: "source folder %s (%s) does not exist", NL: "bronmap %s (%s) bestaat niet"},
		"msg.notRotated":      {EN: "Old backups were not deleted.", NL: "Oude back-ups zijn niet verwijderd."},
		"msg.healthWarning":   {EN: "Disk health: Warning.", NL: "Gezondheid van de schijf: waarschuwing."},
		"msg.healthUnhealthy": {EN: "Disk health: Unhealthy. Replace this disk.", NL: "Gezondheid van de schijf: slecht. Vervang deze schijf."},
		"msg.ejected":         {EN: "Verification OK. %s was ejected; you can unplug it.", NL: "Controle OK. %s is veilig verwijderd; je kunt hem loskoppelen."},
		"msg.ejectFailed":     {EN: "Verification OK, but %s is still in use and could not be ejected. Do not unplug it yet; use \"Safely remove\".", NL: "Controle OK, maar %s is nog in gebruik en kon niet veilig worden verwijderd. Koppel hem nog niet los; gebruik \"Veilig verwijderen\"."},
		"msg.canRemove":       {EN: "Verification OK. You can remove the disk.", NL: "Controle OK. Je kunt de schijf verwijderen."},
		"msg.warnings":        {EN: "backup made, with warnings", NL: "back-up gemaakt, met waarschuwingen"},
		"report.title":        {EN: "USB Backup report", NL: "USB Backup rapport"},
		"report.status":       {EN: "Status", NL: "Status"},
		"report.disk":         {EN: "Disk", NL: "Schijf"},
		"report.period":       {EN: "Period", NL: "Periode"},
		"report.folder":       {EN: "Folder", NL: "Map"},
		"report.started":      {EN: "Started", NL: "Gestart"},
		"report.finished":     {EN: "Finished", NL: "Klaar"},
		"report.encrypted":    {EN: "Encrypted", NL: "Versleuteld"},
		"report.keys":         {EN: "Keys", NL: "Sleutels"},
		"report.health":       {EN: "Health", NL: "Gezondheid"},
		"report.free":         {EN: "Free space", NL: "Vrije ruimte"},
		"report.folders":      {EN: "Folders", NL: "Mappen"},
		"report.rotated":      {EN: "Deleted", NL: "Verwijderd"},
		"report.error":        {EN: "Error", NL: "Fout"},
		"report.archive":      {EN: "Archive", NL: "Archief"},
		"report.files":        {EN: "Files", NL: "Bestanden"},
		"report.size":         {EN: "Size", NL: "Grootte"},
		"report.zipSize":      {EN: "Zip size", NL: "Zipgrootte"},
		"report.verify":       {EN: "Verified", NL: "Controle"},
		"report.skipped":      {EN: "Skipped", NL: "Overgeslagen"},
		"report.warnings":     {EN: "Warnings", NL: "Waarschuwingen"},
		"report.yes":          {EN: "yes", NL: "ja"},
		"report.no":           {EN: "no", NL: "nee"},
		"status.ok":           {EN: "OK", NL: "OK"},
		"status.warning":      {EN: "OK with warnings", NL: "OK met waarschuwingen"},
		"status.failed":       {EN: "FAILED", NL: "MISLUKT"},
		"status.skipped":      {EN: "skipped", NL: "overgeslagen"},
		"status.dryRun":       {EN: "dry run", NL: "proefrun"},
		"status.empty":        {EN: "empty folder, no zip", NL: "lege map, geen zip"},
		"dry.size":            {EN: "Expected size %s, needed %s, free %s", NL: "Verwachte grootte %s, nodig %s, vrij %s"},
		"dry.noRotate":        {EN: "Rotation would delete nothing.", NL: "De rotatie zou niets verwijderen."},
		"dry.rotate":          {EN: "Rotation would delete: %s", NL: "De rotatie zou verwijderen: %s"},
		"dry.nothingWritten":  {EN: "Dry run: nothing was written.", NL: "Proefrun: er is niets geschreven."},
	})
}

func init() {
	add(map[string]entry{
		"err.spaceAfterDelete": {EN: "not enough free space: %s needed, %s free, after deleting %d old folders (the newest older backup was kept)", NL: "te weinig vrije ruimte: %s nodig, %s vrij, na het verwijderen van %d oude mappen (de nieuwste oudere back-up is bewaard)"},
		"msg.plaintextLeft":    {EN: "An unencrypted archive from earlier in this period is still in %s; delete it if it should not stay readable.", NL: "Er staat nog een onversleuteld archief van eerder in deze periode in %s; verwijder het als het niet leesbaar mag blijven."},
	})
}

func init() {
	add(map[string]entry{
		"err.tampered": {EN: "settings or registrations were changed without the operator (signature does not match); nothing was done", NL: "instellingen of registraties zijn gewijzigd zonder beheerder (handtekening klopt niet); er is niets gedaan"},
	})
}

func init() {
	add(map[string]entry{
		"err.busy": {EN: "another backup is running; try again later", NL: "er loopt al een back-up; probeer het later opnieuw"},
		"err.lock": {EN: "the single-instance lock cannot be taken", NL: "de vergrendeling tegen dubbele start lukt niet"},
	})
}
