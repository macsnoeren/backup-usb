package msg

func init() {
	add(map[string]entry{
		"err.otherVault": {EN: "this vault does not belong to this installation (its signing key differs)", NL: "deze vault hoort niet bij deze installatie (de ondertekeningssleutel verschilt)"},
		"cfg.changed":    {EN: "%s changed from %s to %s.", NL: "%s gewijzigd van %s naar %s."},
		"cfg.unsigned":   {EN: "Note: no keys yet, so the settings are not signed. \"keys init\" signs them.", NL: "Let op: er zijn nog geen sleutels, dus de instellingen zijn niet ondertekend. \"keys init\" ondertekent ze."},
	})
}
