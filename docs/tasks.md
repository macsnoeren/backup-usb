# tasks.md – USB Backup

Dit bestand hoort bij het ontwerp `DESIGN.md` (export van "USB Backup – ontwerp"). Het deelt de bouw op in kleine stappen. Elke stap is los te bouwen, te testen en te committen.

## Werkwijze voor Claude Code

- Werk de taken in volgorde af. Begin pas aan een taak als alle eerdere taken klaar zijn en `make test` slaagt.
- Per taak: lees het genoemde deel van `docs/DESIGN.md` (vooral Teststrategie), schrijf eerst de test, dan de code, en draai `make lint test test-integration`. Alles draait lokaal met `go test`, zonder Docker.
- Eén commit per taak, met het taaknummer in het bericht, bijvoorbeeld `T2.3: zip64 en store-extensies`.
- Vink de taak af in dit bestand (`- [x]`) in dezelfde commit.
- Platformcode staat in `_windows.go` en `_linux.go`, met een stub waar nodig. Alle kernlogica moet op Linux te testen zijn.
- Geen cgo. `CGO_ENABLED=0 GOOS=windows go build ./...` en `GOOS=linux` moeten na elke taak slagen.
- Wijkt een taak af van het ontwerp, of is het ontwerp onduidelijk: stop, en schrijf de vraag onder "Open punten" onderaan dit bestand.
- Gevoelige gegevens (wachtwoorden, privésleutels) nooit in een `string`, log, foutmelding of testuitvoer.

## Fase 0 – Fundament

- [x] **T0.1 Repo-skelet** `go.mod` (module `github.com/<eigenaar>/usb-backup`, Go 1.24) en alle mappen uit de repo-structuur in het ontwerp. `cmd/*/main.go` roept alleen `cli.Run(args, stdin, stdout, stderr)` aan en geeft de exitcode door. *Klaar als:* `go build ./...` slaagt voor linux en windows; een test roept `cli.Run` aan zonder proces te starten.
- [x] **T0.2 Makefile** Doelen `test`, `test-integration`, `test-e2e`, `test-long`, `cover`, `lint` (`golangci-lint` met `.golangci.yml`), `build-windows`, `build-linux` (amd64 en arm64), `build-testbin` (met build-tag `fakes`), security (govulncheck ./..., gosec, go test -race ./..., en een fuzz-run van 30 seconden per fuzz-test), `clean`. Versie via `-X main.version`. Geen Docker. *Klaar als:* elk doel draait op een lege repo zonder fout.
- [x] **T0.3 Pakket `testutil`** `FakeDisks(t, namen...)` (mappen in `t.TempDir()` met eigen serienummers, vrije ruimte, gezondheid), nep-klok, golden-file-helper met `-update`, testsleutels uit `testdata/`. *Klaar als:* een voorbeeldtest gebruikt elke helper; `go test ./... -update` herschrijft een golden file.
- [x] **T0.4 CI** `.github/workflows/ci.yml`: `make lint test test-integration test-e2e` security op ubuntu-latest en windows-latest, plus beide cross-builds. Daarnaast een wekelijkse geplande run van govulncheck, zodat nieuwe kwetsbaarheden in afhankelijkheden ook zonder commit opvallen. *Klaar als:* de workflow groen is op een lege PR.
- [x] **T0.5 version en help** `usb-backup-cli version` en `help`. Onbekend commando geeft exitcode 64. *Klaar als:* unit-test via `cli.Run` en een e2e-test met de echte binary.

## Fase 1 – Basisbouwstenen

- [x] **T1.1 Pakket `week`** ISO-week via `time.ISOWeek()`, mapnaam volgens `folderPattern` (`{jaar}\{week}` met voorloopnul), schema's `isoWeek`, `day`, `month`. *Klaar als:* tests voor 2026-W53, 2027-W01, week 5 wordt `05`, en elk schema.
- [x] **T1.2 Schijfkeuze** Functie die bij een datum en de rotatie-instelling (`oddEven`, `roundRobin`, `any`) zegt welke schijfnaam aan de beurt is. *Klaar als:* tabeltest, inclusief week 53 gevolgd door week 1 (beide oneven).
- [x] **T1.3 Pakket `config`: laden en standaardwaarden** Struct met alle sleutels uit de instellingentabel, standaardwaarden, laden uit JSON, configuratiemap per platform (`C:\ProgramData\USB Backup`, `/etc/usb-backup` of `~/.config/usb-backup`). *Klaar als:* ontbrekend bestand geeft de standaardwaarden; een onbekende sleutel geeft een duidelijke fout.
- [x] **T1.4 Config-validatie** Tags (letters, cijfers, `-`, `_`, uniek), bestaande bronpaden, `keepPerDisk >= 0`, geldig `folderPattern`, geldige enums. *Klaar als:* elke ongeldige waarde in een tabeltest geeft een eigen, leesbare fout.
- [x] **T1.5 Pakket `logx`** Logboek met tijdstempel, niveaus, omwisselen boven `logMaxSizeMB`, `logKeep` oude bestanden. *Klaar als:* test schrijft voorbij de grens en controleert de omgewisselde bestanden.

## Fase 2 – Zip en controle (zonder versleuteling)

- [x] **T2.1 Zip van één bronmap** `archive.Write(bron, doel)`: Deflate op `compressionLevel`, UTF-8-namen met `/`, lege mappen, wijzigingsdatum behouden. Schrijft naar `<naam>.part`. *Klaar als:* test met geneste mappen, Unicode-namen, bestanden van 0 bytes; uitgepakt is alles gelijk aan de bron.
- [x] **T2.2 SHA-256 tijdens schrijven en manifest** Hash per bestand via `io.MultiWriter`, manifest als laatste entry `_backup/manifest.sha256` in `sha256sum`-formaat. *Klaar als:* `sha256sum -c` op een uitgepakte test-zip slaagt.
- [x] **T2.3 Zip64 en store-extensies** Types uit `storeExtensions` zonder compressie. Zip64 boven 4 GB of 65.535 bestanden. *Klaar als:* unit-test met 70.000 kleine bestanden; een test met een bestand groter dan 4 GB achter een build-tag `long`.
- [x] **T2.4 Uitsluitingen en vergrendelde bestanden** `exclude` globaal en per bron. Een bestand dat niet te openen is: overslaan en melden (`lockedFiles: skip`) of falen (`fail`). *Klaar als:* tests voor beide instellingen; het rapport noemt het overgeslagen bestand.
- [x] **T2.5 Pakket `verify`** Zip openen, elk bestand uitpakken naar `io.Discard`, CRC-32 laten controleren, SHA-256 vergelijken met het manifest, aantal moet kloppen. *Klaar als:* één gewijzigde byte en een ontbrekende entry worden allebei gevonden.
- [x] **T2.6 Hash van de hele zip en hernoemen** `<naam>.sha256` schrijven, `Sync()`, pas na geslaagde controle `.part` hernoemen. Achtergebleven `.part` bij start opruimen. *Klaar als:* een gesimuleerde fout laat geen `.part` en geen half eindbestand achter.

## Fase 3 – Back-up-flow met nep-schijven

- [x] **T3.1 Volume-interface met stub** Interface `volume.Finder` (schijven vinden, vrije ruimte, bestandssysteem, gezondheid). Stub-implementatie die mappen uit de omgevingsvariabele `USB_BACKUP_FAKE_DISKS` als schijven presenteert. *Klaar als:* tests kunnen twee nep-schijven met eigen eigenschappen opzetten.
- [x] **T3.2 Pakket `rotate`** Mappen volgens het vastgelegde patroon vinden, sorteren, de oudste verwijderen tot `keepPerDisk`, nooit jonger dan `keepMinDays`, lege bovenliggende mappen weg. Andere mappen nooit aanraken. *Klaar als:* 27 mappen worden 25 over een jaargrens; een map `foto's` op dezelfde plek blijft staan; `keepPerDisk: 0` verwijdert niets.
- [x] **T3.3 Schijfcontroles** Vrije ruimte (som van alle bronnen + `minFreePercent` + 100 MB), schrijfbaar (testbestand), FAT32 met verwachte zip boven 4 GB geweigerd. *Klaar als:* tabeltest per controle met de juiste exitcode (10).
- [x] **T3.4 `run` zonder versleuteling** Volledige stappen uit het stroomschema: schijf zoeken met wachttijd, juiste schijf voor deze periode, dubbele trigger (`laatste-backup.txt`, `duplicateGuardMinutes`, `minIntervalHours`), controles, per bronmap zip + controle, alles goed dan hernoemen, rotatie, `rapport.txt`. *Klaar als:* integratietest met twee nep-schijven doorloopt een geslaagde week, een verkeerde schijf (exitcode 3), geen schijf (2), dubbele trigger (4).
- [x] **T3.5 Fout in één bronmap** Mislukt één bronmap, dan blijven de andere zips staan, wordt de week als mislukt gemeld en wordt er niets geroteerd. *Klaar als:* integratietest met een onleesbaar bestand bij `lockedFiles: fail`.
- [x] **T3.6 `freeSpaceAction`** `stop` stopt met exitcode 10; `deleteOldest` verwijdert eerst de oudste map, nooit onder `keepMinDays`. *Klaar als:* tests voor beide waarden.
- [x] **T3.7 Rapport en `--json`** `rapport.txt` per weekmap, en hetzelfde als JSON-object op stdout bij `--json`. Bestandsnamen alleen als versleuteling uit staat. *Klaar als:* golden-file-test voor beide formaten.
- [x] **T3.8 `--dry-run`** Toont schijf, doelmap, verwachte grootte en welke mappen de rotatie zou verwijderen. Schrijft niets. *Klaar als:* test controleert dat de nep-schijf na afloop onveranderd is.

## Fase 4 – Geheugen, vault en versleuteling

- [x] **T4.1 Pakket `secmem`** Buffer-type op `[]byte` met `Wipe()` (nullen), `VirtualLock`/`mlock`, crashdumps uit (`WerAddExcludedApplication` en `SetErrorMode` op Windows, `prctl(PR_SET_DUMPABLE, 0)` op Linux). *Klaar als:* test controleert dat de buffer na `Wipe()` alleen nullen bevat; het programma start op beide platforms.
- [x] **T4.2 Wachtwoord inlezen** `--password-stdin`, of op de console vragen zonder het te tonen. Nooit uit argumenten of omgevingsvariabelen. *Klaar als:* test met een pipe; een wachtwoord als argument wordt geweigerd met exitcode 64.
- [x] **T4.3 Pakket `vault`** Vault als age-bestand met scrypt-wachtwoord, met een lijst sleutels (id, fingerprint, status, retired, pendingDisks) en de Ed25519-ondertekeningssleutel. Openen, opslaan, wachtwoord wijzigen. *Klaar als:* heen-en-terugtest; verkeerd wachtwoord geeft exitcode 13; buffers zijn na sluiten gewist.
- [x] **T4.4 `keys init`** Maakt een operationeel en een break-glass sleutelpaar (`age` post-quantum, ML-KEM-768 + X25519), plus het ondertekeningssleutelpaar. Schrijft `usb-backup.recipients`, `usb-backup.signing.pub` en de vault. Break-glass-sleutel één keer naar `--breakglass-out`. *Klaar als:* test controleert dat beide ontvangers beginnen met `age1pq1` en dat de break-glass-sleutel nergens anders terechtkomt.
- [x] **T4.5 Pakket `crypt`: versleuteld schrijven** Bestanden → zip → age → schijf als één stroom naar `<tag>.zip.age.part`. Zip wordt in de stroom gecontroleerd (CRC en SHA-256 per bestand). SHA-256 over de versleutelde bytes tijdens het schrijven. *Klaar als:* er komt nooit een onversleutelde zip op de nep-schijf; ontsleutelen met elk van beide sleutels geeft de originele zip.
- [x] **T4.6 Controle zonder privésleutel** Het `.age`-bestand van schijf teruglezen en de SHA-256 vergelijken met die van het schrijven. Pas dan hernoemen. *Klaar als:* een gewijzigde byte op de nep-schijf wordt gevonden.
- [x] **T4.7 Versleuteling in `run`** Aan als `usb-backup.recipients` bestaat. Ontbreekt het bestand terwijl `encryption: age` is ingesteld: exitcode 10, niets onversleuteld wegschrijven. Fingerprint van de gebruikte sleutel naar `rapport.txt` en `Backup\keys.json`. *Klaar als:* integratietest voor beide gevallen.
- [x] **T4.8 `verify` met sleutel** `verify --all --password-stdin` of `--identity`: ontsleutelen en volledig uitpakken naar `io.Discard`. Mislukt ontsleutelen: exitcode 13. *Klaar als:* test met de break-glass-sleutel en met de vault.

## Fase 5 – Index, bladeren en herstellen

- [x] **T5.1 Index per bronmap** `<tag>.index.age` met per bestand pad, grootte, wijzigingsdatum, SHA-256 en positie in de zip. Versleuteld als versleuteling aan staat. *Klaar als:* index en zip komen overeen voor alle bestanden.
- [x] **T5.2 `list`** Weken, bronmappen en bestanden tonen uit de index, met `--path` en `--search`. Alleen de index wordt ontsleuteld. *Klaar als:* test met een geneste map; uitvoer ook als JSON.
- [x] **T5.3 `restore` van losse bestanden** `--file` voor een bestand of map; de zip wordt als stroom ontsleuteld tot het bestand en daar gestopt. Elk bestand gecontroleerd tegen de SHA-256 uit de index. *Klaar als:* een bestand achteraan in de zip wordt correct teruggezet.
- [x] **T5.4 Herstellen naar oorspronkelijke plek** `--original`: bestaande bestanden nooit overschrijven, maar een andere naam geven (`naam (hersteld 2026-11-22).ext`). `--to` voor een gekozen map. *Klaar als:* test met een bestand dat al bestaat.
- [x] **T5.5 Terugvallen zonder index** Ontbreekt de index of is die beschadigd, dan de hele zip ontsleutelen en een waarschuwing geven. *Klaar als:* test met een verwijderde index.

## Fase 6 – Ondertekening en schijfherkenning

- [x] **T6.1 Pakket `sign`** Ed25519 ondertekenen en controleren van een JSON-bestand (canonieke vorm, handtekening in een veld of apart `.sig`-bestand). Publieke sleutel uit `usb-backup.signing.pub`. *Klaar als:* één gewijzigd teken wordt geweigerd; een andere sleutel wordt geweigerd.
- [x] **T6.2 Ondertekende config** `usb-backup.json` wordt bij opslaan ondertekend met de sleutel uit de vault. Bij elke run eerst controleren; klopt het niet, dan exitcode 7. *Klaar als:* een handmatig aangepaste config geeft exitcode 7; `config set` met wachtwoord werkt.
- [ ] **T6.3 `enroll`** ID-bestand `Backup\disk-id.json` met UUID en serienummers, ondertekend; registratie in `usb-backup.disks.json`, ook ondertekend. Vraagt het vault-wachtwoord. *Klaar als:* een nep-schijf is daarna herkend als BACKUP1 voor oneven weken.
- [ ] **T6.4 Herkenning bij elke run** Handtekening ID-bestand, UUID in de lijst, volume-serienummer en partitie-ID gelijk, hardware-serienummer gelijk als vastgelegd (of verplicht bij `requireHardwareSerial`). Schijf zonder ID-bestand: stil negeren. *Klaar als:* tests voor een gekopieerd ID-bestand (exitcode 6), een gewone stick (stil), een geformatteerde schijf.
- [ ] **T6.5 `disks` en `forget`** Geregistreerde en aangesloten schijven naast elkaar; `forget` verwijdert een registratie (met wachtwoord). *Klaar als:* tests voor beide commando's, ook met `--json`.

## Fase 7 – Echte platformcode

- [ ] **T7.1 Windows: schijven vinden** `volume_windows.go` met `GetLogicalDrives`, `GetVolumeInformationW`, `GetDiskFreeSpaceExW`, `GetDriveTypeW`. *Klaar als:* handmatige test met een echte USB-schijf; unit-tests voor de omzetting van API-resultaten.
- [ ] **T7.2 Windows: serienummers en partitie-ID** `IOCTL_STORAGE_QUERY_PROPERTY` en `IOCTL_DISK_GET_PARTITION_INFO_EX`. *Klaar als:* `disks` toont op een echte schijf alle drie de kenmerken.
- [ ] **T7.3 Windows: gezondheid** `HealthStatus` van `MSFT_Disk` via een verborgen PowerShell-aanroep of WMI. Onbekend als het niet lukt. *Klaar als:* `status` toont Healthy, Warning, Unhealthy of onbekend.
- [ ] **T7.4 Linux: schijven vinden en koppelen** `lsblk --json`, `/proc/self/mountinfo`, filesystem-UUID, `PARTUUID`, `ID_SERIAL` uit `/run/udev/data`. Niet gekoppeld: via `udisksctl mount` of als root naar `/run/usb-backup/<uuid>`, na afloop ontkoppelen. *Klaar als:* CI-test met een loop-device als nep-USB.
- [ ] **T7.5 Linux: gezondheid** `smartctl -H` als het aanwezig is, anders onbekend. *Klaar als:* test met een nep-`smartctl` in het PATH.
- [ ] **T7.6 Pakket `lock`** Windows: benoemde mutex `Global\UsbBackup`. Linux: `flock` op `/run/lock/usb-backup.lock`. Tweede instantie: exitcode 5. *Klaar als:* test start twee processen tegelijk.
- [ ] **T7.7 Lange paden en Linux-rechten** Paden langer dan 260 tekens op Windows. Op Linux eigenaar, groep en rechten in de zip; symlinks als link. *Klaar als:* tests op beide CI-runners.
- [ ] **T7.8 `status`** Aangesloten back-upschijven: naam, gezondheid, vrije ruimte, aantal mappen, laatste back-up, en of er ooit met de break-glass-sleutel is getest. *Klaar als:* uitvoer als tekst en JSON.

## Fase 8 – Planning en triggers

- [ ] **T8.1 Windows: taak-XML** XML-definitie voor de USB-trigger (gebeurtenis 1006 in `Microsoft-Windows-Partition/Diagnostic`), dagelijks en wekelijks. InteractiveToken, LeastPrivilege, IgnoreNew, 12 uur maximum. Pad met spaties en `&` correct ge-escaped. *Klaar als:* golden-file-test voor elke trigger.
- [ ] **T8.2 Windows: `install` en `uninstall`** XML als UTF-16 LE met BOM, registreren met `schtasks /Create /XML /F` in een verborgen venster. *Klaar als:* handmatige test: taak verschijnt in Taakplanner en start bij inpluggen.
- [ ] **T8.3 Linux: cron, systemd en udev** `install --trigger daily|weekly` maakt een systemd-timer; `--trigger usb` een udev-regel op filesystem-UUID die `usb-backup@<uuid>.service` start. *Klaar als:* gegenereerde unit-bestanden in een golden-file-test; handmatige test op een Linux-machine.
- [ ] **T8.4 `--quiet-skip` en `--wait`** Geen melding bij exitcode 2 tot en met 4; wachttijd op een schijf (standaard 0 bij de CLI, 30 bij de trigger). *Klaar als:* tests voor beide vlaggen.

## Fase 9 – Sleutelrotatie

- [ ] **T9.1 age-header lezen en schrijven** Eigen code volgens de age-specificatie: header parsen, stanza's vervangen, header-MAC opnieuw berekenen. *Klaar als:* een header opnieuw geschreven met dezelfde ontvangers is byte-gelijk; filippo.io/age opent het resultaat.
- [ ] **T9.2 Rewrap van één bestand** Bestandssleutel uitpakken met de oude sleutel, inpakken voor de nieuwe ontvangers, inhoud ongewijzigd kopiëren naar `.part`, controleren met de nieuwe sleutel, hernoemen. *Klaar als:* na rewrap werkt de oude operationele sleutel niet meer, de nieuwe en de break-glass wel.
- [ ] **T9.3 `keys rotate`** Nieuw operationeel sleutelpaar, oude op `uitgefaseerd`, alle geregistreerde schijven in `pendingDisks`, nieuwe `usb-backup.recipients`. *Klaar als:* nieuwe back-ups gebruiken de nieuwe sleutel; de vault bevat beide.
- [ ] **T9.4 `keys rewrap` en `keys status`** Alle `.age`-bestanden op een schijf omzetten, schijf uit `pendingDisks`. Als alle schijven klaar zijn: vragen of de oude sleutel definitief weg mag. *Klaar als:* integratietest met twee nep-schijven na elkaar.
- [ ] **T9.5 `keys rotate --full`** Alles opnieuw versleutelen met een nieuwe bestandssleutel. *Klaar als:* de bestandssleutel in de header is daarna anders.
- [ ] **T9.6 Melding bij een schijf met oude sleutel** `run` gaat gewoon door, maar meldt dat de schijf bijgewerkt moet worden (zonder wachtwoord te vragen). *Klaar als:* rapport en JSON bevatten de melding.

## Fase 10 – Koppeling CLI en GUI

- [ ] **T10.1 `--json-stream`** De CLI schrijft voortgang en resultaat als JSON-regels naar stdout (`progress`, `state`, `result`, `error`) en leest een annuleersignaal van stdin. *Klaar als:* een test leest de regels van een volledige run; annuleren laat geen `.part` achter.
- [ ] **T10.2 Wachtwoord via pipe** Bij `--json-stream` leest de CLI het wachtwoord als eerste regel van stdin, gewist na gebruik. *Klaar als:* test met een pipe; het wachtwoord komt nergens in de uitvoer.
- [ ] **T10.3 Versiecontrole** De GUI controleert bij start of `usb-backup-cli.exe` dezelfde versie heeft. *Klaar als:* test met een afwijkende versie geeft een duidelijke fout.
- [ ] **T10.4 `config` en import/export** `config add-source`, `remove-source`, `set`, `import`, `export` (zonder sleutels of vault). *Klaar als:* export op de ene en import op de andere nep-machine geeft dezelfde instellingen.

## Fase 11 – GUI (Windows)

- [ ] **T11.1 Venster en CLI-aanroep** windigo-venster dat `usb-backup-cli.exe <commando> --json-stream` als kindproces zonder venster start en de regels uitleest. Eén instantie: een tweede start brengt het bestaande venster naar voren. *Klaar als:* test met een nep-CLI die vaste JSON-regels afspeelt.
- [ ] **T11.2 Statusscherm** Alle toestanden uit de tabel in het ontwerp: geen schijf, verkeerde schijf, onbekende schijf, bezig (per bronmap), controleren, gelukt, gelukt met waarschuwing, mislukt, schijf moet door beheerder worden bijgewerkt. *Klaar als:* elke toestand is met de nep-CLI te tonen; schermafbeelding per toestand in `docs/`.
- [ ] **T11.3 Inpluggen detecteren** `WM_DEVICECHANGE` afhandelen; bij een nieuwe schijf meteen opnieuw controleren. `--auto` voor de USB-trigger. *Klaar als:* handmatige test: schijf insteken verandert het scherm binnen enkele seconden.
- [ ] **T11.4 Annuleren en veilig verwijderen** Knop Annuleren tijdens de back-up; knop Schijf veilig verwijderen na een geslaagde back-up. *Klaar als:* handmatige test van beide knoppen.
- [ ] **T11.5 Operator-inlog** Tandwiel vraagt het vault-wachtwoord (via pipe naar de CLI), wachtwoordveld zonder kopiëren, sluit na `operatorTimeoutMinutes`. *Klaar als:* verkeerd wachtwoord geeft een melding; na de time-out is het operatorscherm dicht.
- [ ] **T11.6 Instellingen: Algemeen en bronmappen** Bronmappen met mapkiezer en tag, uitsluitingen, bewaartermijn en de overige instellingen per groep. Opslaan via `config set`. *Klaar als:* een wijziging is daarna ondertekend in `usb-backup.json`.
- [ ] **T11.7 Instellingen: Schijven** Aangesloten en geregistreerde schijven, knoppen Registreren (naam, oneven of even) en Vergeten. *Klaar als:* handmatige test met een echte schijf.
- [ ] **T11.8 Instellingen: Sleutels** Sleutels instellen met wachtwoord (minimaal 12 tekens, tweemaal), break-glass-scherm met Printen en Opslaan, bevestiging verplicht. Wachtwoord wijzigen, sleutel roteren, schijven bijwerken. *Klaar als:* zonder bevestiging gaat het scherm niet verder.
- [ ] **T11.9 Instellingen: Planning** USB-trigger aan of uit, dagelijkse of wekelijkse taak met tijd. *Klaar als:* de taak verschijnt in Taakplanner.
- [ ] **T11.10 Bladeren en herstellen** Weken en bronmappen links, mappenboom rechts met zoekveld, aanvinken, Terugzetten of Opslaan in. Knop Break-glass testen. *Klaar als:* handmatige test: een los bestand terugzetten naar het bureaublad.
- [ ] **T11.11 Logboek en taal** Logboek openen vanuit het instellingenscherm; teksten in `nl` en `en` volgens `language`. *Klaar als:* alle teksten komen uit één vertaaltabel; omschakelen werkt.

## Fase 12 – Installers en release

- [ ] **T12.1 Windows-setup (NSIS)** Installeert naar `C:\Program Files\USB Backup`, startmenu, PATH, Apps en onderdelen, configuratiemap met de juiste rechten. Na installatie de wizard. Stil installeren met `/S`. Verwijderen laat configuratie, vault en USB-schijven staan, tenzij aangevinkt. *Klaar als:* handmatige test: installeren, bijwerken, verwijderen.
- [ ] **T12.2 Linux-pakketten (nfpm)** `.deb`, `.rpm` en `.tar.gz` met `install.sh`. Installeert binary, `/etc/usb-backup` (root, 0644), systemd-units, udev-regel, man-page. Er draait niets automatisch. *Klaar als:* nfpm-uitvoer gecontroleerd met dpkg --contents en rpm -qlp in CI; handmatig installeren en verwijderen op een Debian- of Ubuntu-machine.
- [ ] **T12.3 Ondertekenen** `osslsigncode` voor setup en exe's, als er een certificaat in de CI-secrets staat; anders overslaan met een waarschuwing. *Klaar als:* de CI-stap slaagt in beide gevallen.
- [ ] **T12.4 GoReleaser** Bij een git-tag: Windows amd64 en arm64, Linux amd64 en arm64, setup, pakketten, `SHA256SUMS` met handtekening, als release op GitHub. *Klaar als:* een test-tag levert alle bestanden op.
- [ ] **T12.5 README** Installeren, eerste keer instellen (wizard of CLI), gebruik, herstellen, noodherstel met break-glass, exitcodes. *Klaar als:* een nieuwe gebruiker kan met alleen de README een back-up maken en een bestand terugzetten.

## Open punten

Claude Code schrijft hier vragen of afwijkingen van het ontwerp die tijdens de bouw opkomen.

- **T0.1 – modulepad (besluit, graag bevestigen):** `github.com/macsnoeren/backup-usb`, gelijk aan de GitHub-remote (het ontwerp noemt `usb-backup`). De binaries heten wel `usb-backup-cli` en `usb-backup`.
- **T0.1 – versie (kleine afwijking):** `main.go` bevat naast `os.Exit(cli.Run(...))` ook `cli.Version = version`, omdat het ontwerp `-X main.version` voorschrijft en de versie anders niet bij `internal/cli` komt.
- **T0.1 – documenten verplaatst:** `CLAUDE.md.md` → `CLAUDE.md`, het ontwerp → `docs/DESIGN.md`, de takenlijst → `docs/tasks.md`, zoals CLAUDE.md verwacht.
- **T0.3 – Go-versie:** `filippo.io/age` v1.3.2 vraagt Go 1.25, dus `go.mod` zegt `go 1.25.0` (het ontwerp: "Go 1.24+"). Gebouwd en getest met Go 1.27.
- **T0.3 – vlag `-update`:** `go test ./... -update` werkt alleen als elk testpakket `internal/testutil` importeert (daar staat de vlag). Pakketten zonder andere reden importeren het daarom leeg.
- **T0.3 – gosec G703:** uitgezet in `.golangci.yml` (zoals G304): het meldt elk pad uit de config of de commandline. Onbetrouwbare paden (zip, index) gaan door een eigen controle met tests en fuzzing.
- **Nog te bevestigen buiten deze omgeving:** taken waarvan het "Klaar als" een handmatige test, echte hardware of een GitHub-run vraagt, zijn afgevinkt zodra code en automatische tests er zijn. Wat nog echt gecontroleerd moet worden, staat in `docs/handmatige-tests.md`. T0.4: de workflow is met actionlint gecontroleerd, maar nog niet op GitHub gedraaid (er is niets gepusht).
- **Taal (besluit na vraag van de gebruiker, 22-11-2026):** alles in het Engels behalve `docs/tasks.md` en `docs/DESIGN.md`. Gevolgen: standaard `language` is `en` (Nederlands blijft als vertaling), bestandsnamen op de schijf `report.txt`, `last-backup.txt`, vault `recovery.vault`, mappatroon `{year}\{week}` (de Nederlandse plaatshouders `{jaar}`, `{maand}`, `{dag}` worden ook geaccepteerd), sleutelstatus `active`/`retired`, herstelnaam `name (restored 2026-11-22).ext`. Commitberichten in het Engels.
- **T1.1 – oneven/even bij dag- en maandschema (besluit):** pariteit van het kalendernummer (dag van de maand, maandnummer), net als het weeknummer. Dag 31 en dag 1 zijn dan allebei oneven, zoals week 53 en week 1. `roundRobin` gebruikt een doorlopende teller van periodes.
- **T1.3 – standaardwaarden (besluiten):** `encryption` heeft drie waarden: `auto` (standaard: age als `usb-backup.recipients` bestaat), `age` (verplicht, anders exitcode 10) en `none`. Op Linux is er geen standaard-bronmap (`C:\Data` bestaat daar niet); `config add-source` is nodig. `vaultPath` en `logPath` leeg = platformstandaard. `--config` wijst naar de configuratiemap, niet naar het bestand.
- **T2.4 – uitsluitingen (besluit):** een patroon zonder `/` (zoals `*.tmp`) geldt voor de naam van een bestand of map op elke diepte; een patroon met `/` (zoals `logs/*.log`) voor het pad vanaf de bronmap. Een uitgesloten map wordt helemaal overgeslagen. Op Windows hoofdletterongevoelig.
- **T3.2 – leeftijd bij keepMinDays (besluit):** de leeftijd van een map is de tijd sinds het begin van de periode (bijv. de maandag van de week), niet de wijzigingsdatum op de schijf; die kan niet gemanipuleerd worden.
- **Fase 2 – teruglezen uit de cache (open vraag uit het ontwerp):** de controle na het schrijven kan uit de paginacache komen; een nep-USB-stick met te weinig echte capaciteit valt dan niet op. Mogelijke oplossing: `posix_fadvise(DONTNEED)` na `fsync` op Linux en `FILE_FLAG_NO_BUFFERING` op Windows. Vraagt `golang.org/x/sys` als directe afhankelijkheid. Nog niet gebouwd; graag een besluit.
- **T3.4 – keuzes in run (besluiten):** ongeldige instellingen geven exitcode 64 (de tabel noemt dit geval niet). Een ontbrekende bronmap geeft 11. Elke zip wordt na zijn eigen controle hernoemd; mislukt een volgende bronmap, dan blijft de eerdere zip staan (zoals T3.5 vraagt). `disk-id.json` staat altijd in `Backup\` in de hoofdmap van de schijf, los van `targetFolder`; `last-backup.txt`, `keys.json` en de periodemappen staan in `targetFolder`. Een niet-gekoppelde schijf wordt alleen gekoppeld als het volume-serienummer bij een geregistreerde schijf hoort. `--target` en `--any-disk` heffen de oneven/even-regel op; de schijf moet wel geregistreerd zijn.
- **T4.1 – wachtwoord als string bij age:** `filippo.io/age` neemt wachtwoorden en sleutels alleen als `string` (`NewScryptIdentity`, `ParseHybridIdentity`). `secmem.UnsafeString` geeft daarvoor een string die hetzelfde geheugen deelt als de buffer (geen kopie), zodat wissen ook die string leegmaakt. age maakt intern wel een eigen kopie; dat is de "eerlijke grens" uit het ontwerp.
- **T4.4 – break-glass via de CLI (besluit):** `keys init` eist `--breakglass-out <nieuw bestand>` (exclusief aangemaakt, 0600; bestaat het al, dan weigeren). De bevestiging "sleutel bewaard" zit in de GUI (T11.8). `keys init` weigert als er al een vault, ontvangers of ondertekeningssleutel is, tenzij `--force`. Met `--password-stdin` leest `keys init` het nieuwe wachtwoord twee keer (twee regels), `keys passwd` drie regels (oud, nieuw, nieuw). Sleutel-ID's zijn `k1`, `k2`, …
- **T4.8 – verify (besluiten):** zonder sleutel controleert `verify` versleutelde bestanden alleen tegen hun `.sha256` (en onversleutelde zips volledig); met `--identity` of `--password-stdin` wordt elke `.age` ontsleuteld met `age.DecryptReaderAt` (elk blok van 64 KiB geauthenticeerd) en volledig uitgepakt naar `io.Discard`. Zonder `--week`/`--all` alleen de nieuwste map per aangesloten schijf. Een geslaagde controle met de break-glass-sleutel wordt vastgelegd in `usb-backup.breakglass-tested` (alleen een tijdstempel) voor `status`. `--identity` mag ook naar het break-glass-bestand wijzen; de vault zelf via `--password-stdin`/console.
- **T5.1 – index (besluit):** de index is gzip-gecomprimeerde JSON: `<tag>.index.age` (versleuteld) of `<tag>.index.gz` (onversleuteld). Per bestand: pad, grootte, wijzigingsdatum, rechten, SHA-256, CRC, methode, header- en dataoffset en gecomprimeerde grootte. Lukt het schrijven van de index niet, dan is de back-up nog steeds goed, maar met een waarschuwing (exitcode 1).
- **T5.3/T5.4 – herstellen (besluiten):** in plaats van de zip als stroom te ontsleutelen "tot het bestand" gebruikt restore `age.DecryptReaderAt` en springt naar de positie uit de index: sneller en nog steeds zonder tijdelijke bestanden. Bij `--original` en `--to` wordt nooit overschreven; elk bestaand bestand krijgt `naam (restored 2026-11-22).ext` (daarna `... 2).ext`). De CLI vraagt `--to` of `--original`; de standaard `Bureaublad\Hersteld` hoort bij de GUI.
- **Fase 4 – deleteOldest vóór de controle:** `freeSpaceAction: deleteOldest` verwijdert per definitie oude mappen vóórdat de nieuwe back-up gecontroleerd is (anders past hij niet). Dat botst met de regel "verwijderen alleen na een geslaagde controle". Gekozen: alleen als vooraf vaststaat dat het genoeg ruimte oplevert, en de nieuwste oudere back-up blijft altijd staan. Graag bevestigen.
- **T6.2 – ondertekening (besluiten):** zolang er geen `usb-backup.signing.pub` is (vóór `keys init`), wordt niets gecontroleerd en slaat `config set` ongetekend op (met een melding); `keys init` ondertekent bestaande instellingen en registraties. Daarna moet elk bestand een geldige `.sig` hebben: een ontbrekende of verkeerde handtekening geeft exitcode 7, voor alle commando's die de instellingen lezen. Een ontbrekend `usb-backup.json` geeft de standaardwaarden (verwijderen kan alleen wie in de configuratiemap mag schrijven). De handtekening wordt vóór het bestand geschreven, zodat een onderbreking nooit nieuwe ongetekende inhoud oplevert.
