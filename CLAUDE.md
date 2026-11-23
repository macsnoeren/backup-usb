# CLAUDE.md

USB Backup: versleutelde, gecontroleerde back-ups van mappen naar geregistreerde USB-schijven, voor Windows en Linux. Go, zonder cgo. `usb-backup-cli` doet al het werk; `usb-backup` is de Windows-GUI die de CLI als kindproces aanroept.

## Belangrijke bestanden

- `docs/DESIGN.md`: het ontwerp. Leidend bij elke beslissing.
- `docs/tasks.md`: de bouwvolgorde. Werk taak voor taak, in volgorde.
- Wijk je af van het ontwerp, of is iets onduidelijk: stop en schrijf de vraag onder "Open punten" in `docs/tasks.md`. Niet gokken.

## Commando's

```
make test               # unit-tests (go test ./...)
make test-integration   # build-tag integration, nep-schijven
make test-e2e           # echte binary tegen nep-schijven
make test-long          # zip64 en grote aantallen (langzaam)
make lint               # golangci-lint, inclusief gosec
make security           # govulncheck, gosec, go test -race, fuzz (kort)
make cover              # dekking
make build-windows      # dist/usb-backup-cli.exe en dist/usb-backup.exe
make build-linux        # amd64 en arm64
go test ./... -update   # golden files bijwerken
```

Geen Docker. Alles moet lokaal draaien met `go test`.

## Werkwijze per taak

1. Lees de taak in `docs/tasks.md` en het bijbehorende deel van het ontwerp.
2. Schrijf eerst de test, dan de code.
3. Draai `make lint test test-integration`; bij CLI-gedrag ook `make test-e2e`.
4. Controleer dat `CGO_ENABLED=0 GOOS=windows go build ./...` en `GOOS=linux go build ./...` slagen.
5. **Security-review** volgens de sectie hieronder. Draai `make security` en loop de checklist na over de diff van deze taak.
6. Vink de taak af en maak één commit met het taaknummer: `T2.3: zip64 and store extensions`. Zet in het commitbericht een regel `Security-review: OK` of de gevonden punten.

Een taak is pas klaar als de security-review geen open punten heeft. Een gevonden probleem wordt in dezelfde taak opgelost, met een test die het probleem aantoont. Kan dat niet, dan komt het onder "Open punten" en gaat de taak niet door.

## Codeerregels

- `cmd/*/main.go` blijft dun: alleen `os.Exit(cli.Run(os.Args, os.Stdin, os.Stdout, os.Stderr))`.
- Alle code in `internal/`. Geen `pkg/`.
- Klok, schijven, wachtwoordinvoer en taakregistratie gaan via interfaces; tests gebruiken de nep-versies uit `internal/testutil`.
- Platformcode in `_windows.go` en `_linux.go`. Kernlogica moet op Linux te testen zijn.
- Nep-schijven en nep-klok (`USB_BACKUP_FAKE_DISKS`, `USB_BACKUP_FAKE_NOW`) alleen met build-tag `fakes`, nooit in een productiebuild.
- Afhankelijkheden zo min mogelijk: standaardbibliotheek, `filippo.io/age`, windigo (alleen GUI). Nieuwe afhankelijkheid alleen na een notitie onder "Open punten".
- Fouten met context teruggeven (`fmt.Errorf("...: %w", err)`); exitcodes alleen in `internal/cli`, volgens de tabel in het ontwerp.
- Licentie GPL-3.0-or-later. Elk nieuw bronbestand (`.go`, scripts, Makefile, packaging, workflows) begint met de licentiekop uit een bestaand bestand (SPDX-regel, copyright, GPL-tekst); `license_test.go` controleert dat. Na een wijziging in `go.mod`: `make notices` voor `THIRD-PARTY-NOTICES.txt`.
- Alles in het Engels: code, commentaar, logberichten, gebruikersteksten (via de vertaaltabel `internal/msg`, standaardtaal `en`, Nederlands optioneel), bestandsnamen op de schijf, commitberichten en README. Alleen `docs/tasks.md` en `docs/DESIGN.md` blijven Nederlands.

## Beveiliging: altijd

- Wachtwoorden en privésleutels alleen in `[]byte` uit `internal/secmem`, nooit in een `string`. Na gebruik wissen.
- Nooit gevoelige gegevens in logs, foutmeldingen, `--json`-uitvoer, testuitvoer of tijdelijke bestanden.
- Wachtwoord nooit uit argumenten of omgevingsvariabelen; alleen stdin of een console-prompt.
- Nooit onversleutelde back-updata op een USB-schijf schrijven als versleuteling aan staat.
- Rotatie en verwijderen alleen na een geslaagde controle, en alleen mappen die precies aan het vastgelegde patroon voldoen.
- Eerst naar `.part` schrijven, controleren, dan pas hernoemen.

## Security-review na elke taak

Na elke taak doe je een aparte review van alleen de gewijzigde code (`git diff`), met de blik van een aanvaller. Gebruik bij voorkeur een subagent die de code niet zelf heeft geschreven.

### Automatisch (`make security`)

- `govulncheck ./...`: bekende kwetsbaarheden in Go en afhankelijkheden. Elke melding moet opgelost of onderbouwd zijn.
- `gosec` (ook via golangci-lint): onveilige patronen zoals zwakke random, onveilige bestandsrechten, padproblemen.
- `go test -race ./...`: race conditions, vooral rond de GUI-koppeling, locks en voortgang.
- Korte fuzz-run (`-fuzztime=30s`) op alle parsers: zip-lezen, age-header, index, `disk-id.json`, config, JSON-stream.

### Checklist (handmatig over de diff)

- **Geheimen:** komt een wachtwoord of privésleutel ergens in een `string`, log, foutmelding, `--json`, panic-tekst of tijdelijk bestand? Wordt elke `secmem`-buffer gewist, ook bij een fout (`defer`)?
- **Paden:** kan een bestandsnaam uit een zip of index buiten de doelmap terechtkomen (zip-slip, `..`, absolute paden, Windows-stationsletters, UNC)? Worden symlinks bij herstel niet gevolgd naar buiten de doelmap?
- **Invoer van de USB-schijf is onbetrouwbaar:** zips, indexen, `disk-id.json`, `keys.json` en `rapport.txt` kunnen door een aanvaller gemaakt zijn. Worden groottes begrensd (zip-bom, enorme headers, diepe nesting)? Faalt het veilig?
- **Handtekeningen en versleuteling:** wordt elke handtekening gecontroleerd vóór gebruik van de gegevens? Geen eigen crypto buiten de age-header-rewrap; alleen `crypto/rand`; vergelijkingen van geheimen met `subtle.ConstantTimeCompare`.
- **Verwijderen:** kan rotatie of opruimen ooit iets buiten de back-upmap of buiten het vastgelegde patroon verwijderen? Alleen na een geslaagde controle?
- **Externe processen:** worden `schtasks`, `smartctl`, `lsblk`, `udisksctl` en PowerShell aangeroepen zonder shell, met argumenten als lijst, met een volledig pad? Geen invoer van gebruikers in een commandoregel of XML zonder escaping?
- **Bestandsrechten:** configuratiemap en vault alleen schrijfbaar voor wie dat mag (Windows-ACL, Linux 0600/0644, eigenaar root)? Nieuwe bestanden met beperkte rechten aangemaakt?
- **TOCTOU:** controleer je een bestand en open je het daarna opnieuw? Liever openen en dan controleren op de open handle.
- **Foutafhandeling:** wordt elke fout behandeld? Geen genegeerde `err` bij schrijven, `Sync`, `Close` of hernoemen.
- **Afhankelijkheden:** nieuwe module toegevoegd? Dan eerst onder "Open punten", met de reden en de onderhoudsstatus.

### Verslag

Zet de uitkomst kort in het commitbericht. Voor taken die geheimen, crypto, paden of verwijderen raken (fase 2, 4, 5, 6, 9 en de GUI-koppeling) schrijf je bovendien een korte notitie in `docs/security-review.md`: taaknummer, wat er bekeken is, wat er gevonden en opgelost is.

## Tests

- Tabeltests en `t.TempDir()`; geen echte schijven of netwerk in automatische tests.
- Testsleutels uit `testdata/` en lage scrypt-instellingen, zodat `make test` binnen een minuut klaar is.
- Golden files in `testdata/`; alleen bijwerken met `-update` als de wijziging bedoeld is.
- Tests die rechten nodig hebben (loop-devices) overslaan met `t.Skip` als die er niet zijn.
