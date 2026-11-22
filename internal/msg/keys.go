package msg

func init() {
	add(map[string]entry{
		"err.vaultMissing":  {EN: "vault %s not found; create one with \"keys init\"", NL: "vault %s niet gevonden; maak er een met \"keys init\""},
		"err.vaultPassword": {EN: "wrong vault password", NL: "verkeerd vault-wachtwoord"},
		"err.vaultDamaged":  {EN: "the vault cannot be read", NL: "de vault kan niet worden gelezen"},
	})
}

func init() {
	add(map[string]entry{
		"err.keysExist":       {EN: "%s already exists; keys were made before (use --force only if you know the old keys are no longer needed)", NL: "%s bestaat al; er zijn al sleutels (gebruik --force alleen als de oude sleutels niet meer nodig zijn)"},
		"err.keygen":          {EN: "creating keys failed", NL: "sleutels maken is mislukt"},
		"err.breakglassWrite": {EN: "the break-glass key could not be written; nothing was saved", NL: "de break-glass-sleutel kon niet worden geschreven; er is niets opgeslagen"},
		"err.fileExists":      {EN: "%s already exists; choose a new file", NL: "%s bestaat al; kies een nieuw bestand"},
		"err.passwordInput":   {EN: "password: %s", NL: "wachtwoord: %s"},
		"keys.passwdDone":     {EN: "Vault password changed. The backups are unchanged.", NL: "Vault-wachtwoord gewijzigd. De back-ups zijn niet veranderd."},
		"keys.prompt":         {EN: "Vault password: ", NL: "Vault-wachtwoord: "},
		"keys.promptNew":      {EN: "New vault password (at least 12 characters): ", NL: "Nieuw vault-wachtwoord (minimaal 12 tekens): "},
		"keys.promptAgain":    {EN: "Repeat the new password: ", NL: "Herhaal het nieuwe wachtwoord: "},
		"keys.created":        {EN: "Keys created.\n  vault:       %s\n  recipients:  %s\n  operational: %s (%s)\n  break-glass: fingerprint %s, written to %s\nStore the break-glass key offline now. Without the vault password AND without the break-glass key, the backups are worthless.", NL: "Sleutels gemaakt.\n  vault:       %s\n  ontvangers:  %s\n  operationeel: %s (%s)\n  break-glass: vingerafdruk %s, geschreven naar %s\nBewaar de break-glass-sleutel nu offline. Zonder vault-wachtwoord EN zonder break-glass-sleutel zijn de back-ups waardeloos."},
	})
}

func init() {
	add(map[string]entry{
		"err.noRecipients":  {EN: "encryption is required but %s is missing; nothing was written (run \"keys init\")", NL: "versleuteling is verplicht maar %s ontbreekt; er is niets geschreven (voer \"keys init\" uit)"},
		"err.badRecipients": {EN: "the recipients file %s is invalid; nothing was written", NL: "het ontvangersbestand %s is ongeldig; er is niets geschreven"},
	})
}
