package msg

func init() {
	add(map[string]entry{
		"err.vaultMissing":  {EN: "vault %s not found; create one with \"keys init\"", NL: "vault %s niet gevonden; maak er een met \"keys init\""},
		"err.vaultPassword": {EN: "wrong vault password", NL: "verkeerd vault-wachtwoord"},
		"err.vaultDamaged":  {EN: "the vault cannot be read", NL: "de vault kan niet worden gelezen"},
	})
}
