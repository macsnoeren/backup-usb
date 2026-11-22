package msg

func init() {
	add(map[string]entry{
		"err.needKeysInit":   {EN: "no keys yet: run \"keys init\" first (registrations are signed by the operator)", NL: "nog geen sleutels: voer eerst \"keys init\" uit (registraties worden door de beheerder ondertekend)"},
		"err.diskName":       {EN: "%q is not a valid disk name (1 to 32 letters, digits, - and _)", NL: "%q is geen geldige schijfnaam (1 tot 32 letters, cijfers, - en _)"},
		"err.weeksNeeded":    {EN: "give --weeks odd or --weeks even", NL: "geef --weeks odd of --weeks even"},
		"err.diskRegistered": {EN: "%s is already registered; use \"forget\" first or --force", NL: "%s is al geregistreerd; gebruik eerst \"forget\" of --force"},
		"err.noSuchDisk":     {EN: "no disk %q connected", NL: "geen schijf %q aangesloten"},
		"err.mount":          {EN: "disk %s cannot be mounted", NL: "schijf %s kan niet worden gekoppeld"},
		"err.noSerials":      {EN: "the serial numbers of %s cannot be read", NL: "de serienummers van %s kunnen niet worden gelezen"},
		"disk.enrolled":      {EN: "%s registered for %s periods (ID %s).", NL: "%s geregistreerd voor %s periodes (ID %s)."},
	})
}

func init() {
	add(map[string]entry{
		"err.notRegistered":     {EN: "no registered disk %q", NL: "geen geregistreerde schijf %q"},
		"disk.forgotten":        {EN: "%s is no longer registered. Its backups stay on the disk.", NL: "%s is niet meer geregistreerd. De back-ups blijven op de schijf staan."},
		"disk.registered":       {EN: "Registered disks:", NL: "Geregistreerde schijven:"},
		"disk.connected":        {EN: "Connected disks:", NL: "Aangesloten schijven:"},
		"disk.none":             {EN: "  (none)", NL: "  (geen)"},
		"disk.state.registered": {EN: "registered as %s", NL: "geregistreerd als %s"},
		"disk.state.unknown":    {EN: "unknown disk (ID file does not match)", NL: "onbekende schijf (ID-bestand klopt niet)"},
		"disk.state.plain":      {EN: "not a backup disk", NL: "geen back-upschijf"},
		"disk.isConnected":      {EN: "connected at %s", NL: "aangesloten op %s"},
		"disk.notConnected":     {EN: "not connected", NL: "niet aangesloten"},
	})
}
