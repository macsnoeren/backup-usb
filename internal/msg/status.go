package msg

func init() {
	add(map[string]entry{
		"status.period":       {EN: "Period %s: %s is due.", NL: "Periode %s: %s is aan de beurt."},
		"status.noDisks":      {EN: "No backup disk connected.", NL: "Geen back-upschijf aangesloten."},
		"status.disk":         {EN: "%s (%s): health %s, %s free of %s, %d folders, last backup %s%s", NL: "%s (%s): gezondheid %s, %s vrij van %s, %d mappen, laatste back-up %s%s"},
		"status.never":        {EN: "never", NL: "nooit"},
		"status.dueMark":      {EN: " (due now)", NL: " (nu aan de beurt)"},
		"status.needsUpdate":  {EN: "  This disk must be updated by the operator (keys rewrap).", NL: "  Deze schijf moet door de beheerder worden bijgewerkt (keys rewrap)."},
		"status.bgNever":      {EN: "Warning: the break-glass key has never been tested. Test it with \"verify --identity <break-glass file>\".", NL: "Let op: de break-glass-sleutel is nog nooit getest. Test hem met \"verify --identity <break-glass-bestand>\"."},
		"status.bgTested":     {EN: "Break-glass key last tested: %s.", NL: "Break-glass-sleutel laatst getest: %s."},
		"status.notEncrypted": {EN: "Backups are not encrypted.", NL: "De back-ups zijn niet versleuteld."},
	})
}
