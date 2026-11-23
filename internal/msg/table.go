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

// table is the translation table. Keys are stable identifiers; every key has
// at least an English text. Format verbs must match between languages.
var table = map[string]entry{
	"help.usage": {
		NL: `Gebruik: usb-backup-cli <commando> [vlaggen]

Commando's:
  run          back-up maken van alle bronmappen
  verify       bestaande back-ups opnieuw controleren
  list         bladeren in een back-up
  restore      bestanden terugzetten
  status       aangesloten back-upschijven tonen
  enroll       schijf registreren
  disks        geregistreerde en aangesloten schijven tonen
  forget       registratie van een schijf verwijderen
  keys         sleutels beheren (init, passwd, rotate, rewrap, status)
  config       instellingen beheren
  install      geplande taak registreren
  uninstall    geplande taak verwijderen
  version      versie tonen
  help         deze uitleg, of "help <commando>"

Vlaggen voor alle commando's: --config <map>, --log <map>, --ui, --no-ui, --quiet
Wachtwoorden nooit als argument: gebruik --password-stdin of de vraag op de console.
`,
		EN: `Usage: usb-backup-cli <command> [flags]

Commands:
  run          back up all source folders
  verify       check existing backups again
  list         browse a backup
  restore      restore files
  status       show connected backup disks
  enroll       register a disk
  disks        show registered and connected disks
  forget       remove the registration of a disk
  keys         manage keys (init, passwd, rotate, rewrap, status)
  config       manage settings
  install      register a scheduled task
  uninstall    remove a scheduled task
  version      show the version
  help         this text, or "help <command>"

Flags for all commands: --config <dir>, --log <dir>, --ui, --no-ui, --quiet
Never pass passwords as arguments: use --password-stdin or the console prompt.
`,
	},
	"help.command":         {NL: "Gebruik: usb-backup-cli %s [vlaggen]\n\n%s\n\nVlaggen:\n", EN: "Usage: usb-backup-cli %s [flags]\n\n%s\n\nFlags:\n"},
	"err.unknownCommand":   {NL: "onbekend commando %q; zie \"usb-backup-cli help\"", EN: "unknown command %q; see \"usb-backup-cli help\""},
	"err.usage":            {NL: "verkeerd gebruik: %s", EN: "wrong usage: %s"},
	"err.passwordArgument": {NL: "een wachtwoord mag nooit als argument worden meegegeven; gebruik --password-stdin of de vraag op de console", EN: "a password must never be passed as an argument; use --password-stdin or the console prompt"},
	"version":              {NL: "usb-backup-cli versie %s (%s, %s/%s)", EN: "usb-backup-cli version %s (%s, %s/%s)"},
	"license": {
		EN: "Copyright (C) 2026 Maurice Snoeren\nLicense GPLv3+: GNU GPL version 3 or later <https://www.gnu.org/licenses/gpl-3.0.html>.\nThis is free software: you are free to change and redistribute it.\nThere is NO WARRANTY, to the extent permitted by law.",
		NL: "Copyright (C) 2026 Maurice Snoeren\nLicentie GPLv3+: GNU GPL versie 3 of later <https://www.gnu.org/licenses/gpl-3.0.html>.\nDit is vrije software: je mag het aanpassen en verder verspreiden.\nEr is GEEN GARANTIE, voor zover de wet dat toestaat.",
	},
}

func init() {
	add(map[string]entry{
		"err.write":        {EN: "writing to the backup disk failed", NL: "schrijven naar de back-upschijf is mislukt"},
		"err.read":         {EN: "reading a file failed", NL: "lezen van een bestand is mislukt"},
		"err.locked":       {EN: "a file could not be opened (lockedFiles: fail)", NL: "een bestand kon niet worden geopend (lockedFiles: fail)"},
		"err.cancelled":    {EN: "cancelled", NL: "geannuleerd"},
		"err.fat32":        {EN: "%s uses FAT32, which cannot hold a zip larger than 4 GB; use exFAT or NTFS", NL: "%s gebruikt FAT32, dat geen zip groter dan 4 GB aankan; gebruik exFAT of NTFS"},
		"err.space":        {EN: "not enough free space: %s needed, %s free; nothing was deleted", NL: "te weinig vrije ruimte: %s nodig, %s vrij; er is niets verwijderd"},
		"err.spaceUnknown": {EN: "the free space of the disk cannot be read", NL: "de vrije ruimte van de schijf kan niet worden gelezen"},
		"err.notWritable":  {EN: "the backup disk is not writable", NL: "de back-upschijf is niet schrijfbaar"},
		"err.verify":       {EN: "verification of %s failed; old backups were not deleted", NL: "controle van %s is mislukt; oude back-ups zijn niet verwijderd"},
	})
}

// add merges entries into the table; a duplicate key is a programming
// error.
func add(m map[string]entry) {
	for k, v := range m {
		if _, dup := table[k]; dup {
			panic("msg: duplicate key " + k)
		}
		table[k] = v
	}
}
