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
}
