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
