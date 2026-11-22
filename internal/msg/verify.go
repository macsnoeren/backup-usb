package msg

func init() {
	add(map[string]entry{
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
