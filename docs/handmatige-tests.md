# Handmatige tests

Dingen die alleen met echte hardware, Windows, Taakplanner, udev of de GUI
te controleren zijn. In de bouwomgeving (Linux, zonder Windows-machine en
zonder USB-schijven) zijn deze niet gedaan; ze staan hier als checklist.

## Windows (fase 7)

- [ ] **T7.1** Een echte USB-schijf wordt gevonden: `usb-backup-cli disks` toont schijfletter, label, bestandssysteem, volume-serienummer en vrije ruimte. De systeemschijf ontbreekt.
- [ ] **T7.2** `disks` toont op een echte schijf alle drie de kenmerken: volume-serienummer, partitie-ID (GPT-GUID, of `MBR:<signature>:<nummer>`) en hardware-serienummer. Controleer ook een goedkope stick zonder serienummer (hardware-serienummer leeg).
- [ ] **T7.3** `usb-backup-cli status` toont Healthy, Warning, Unhealthy of Unknown; er verschijnt geen PowerShell-venster.
- [ ] Veilig verwijderen na een back-up (`ejectAfter`, standaard aan) en met de GUI-knop werkt, als **gewone gebruiker**, met een USB-stick én een externe harde schijf of SSD (ook UAS): de schijf verdwijnt uit Verkenner en het venster zegt dat hij los mag. Met een open bestand op de schijf zegt het venster dat hij nog in gebruik is. Andere USB-apparaten blijven werken.
- [ ] Onderaan het venster staat het aantal back-ups en de vrije ruimte (ook na het uitwerpen), leesbaar met één en met twee schijven.
- [ ] Na het uitwerpen en opnieuw insteken wordt de schijf weer gevonden.

## Linux (fase 7)

- [ ] **T7.4** CI-stap "Loop-device tests" (`sudo go test -tags integration,loopdev ./internal/volume/...`) is groen op GitHub (nog niet gedraaid).
- [ ] Een echte USB-stick die niet gekoppeld is, wordt bij `run` via `udisksctl` gekoppeld en daarna weer ontkoppeld (als gewone gebruiker), en als root naar `/run/usb-backup/<uuid>` met `nosuid,nodev,noexec`.
- [ ] **T7.5** `status` toont de gezondheid via `smartctl -H` op een schijf die SMART doorgeeft; zonder smartmontools "Unknown".
- [ ] **T7.7 (Windows)** `TestLongPaths` slaagt op de Windows-CI-runner (pad langer dan 260 tekens, backup en herstel).

## Planning (fase 8)

- [ ] **T8.2** `usb-backup-cli install` (Windows): de taak "USB Backup" verschijnt in Taakplanner, start bij het insteken van een geregistreerde schijf (met de GUI als `usb-backup.exe` ernaast staat) en toont het venster in de sessie van de gebruiker. Een taak "USB Backup (daily)" of "(weekly)" van een oudere versie is na het bijwerken met de setup weg. `uninstall --trigger all` haalt alles weg.
- [ ] **T8.3** Op Linux (als root): `install --trigger usb` maakt de udev-regel en `usb-backup@.service`; insteken van een geregistreerde stick start de service (`journalctl -u 'usb-backup@*'`).

## GUI (fase 11, alleen Windows)

De logica van het statusscherm (alle toestanden, teksten, resterende tijd), de time-out van het operatorscherm en de koppeling met de CLI (versiecontrole, JSON-regels, annuleren, wachtwoord via de pipe) zijn met automatische tests op Linux gedekt (`internal/gui/*_test.go`, nep-CLI). De vensters zelf (windigo) zijn alleen gecompileerd. Te testen op een Windows-pc:

- [ ] **T11.1** `usb-backup.exe` opent het venster; een tweede start brengt het bestaande venster naar voren. Met een `usb-backup-cli.exe` van een andere versie verschijnt een duidelijke foutmelding.
- [ ] **T11.2** Elke toestand verschijnt (geen schijf, verkeerde schijf, onbekende schijf, bezig met bronmap en percentage, controleren, gelukt, gelukt met waarschuwing, mislukt, bijwerken door beheerder). Maak per toestand een schermafbeelding in `docs/screenshots/` (nog niet gemaakt).
- [ ] **T11.3** Schijf insteken verandert het scherm binnen enkele seconden en start de back-up; `usb-backup.exe --auto` (USB-trigger) start direct.
- [ ] **T11.4** Annuleren tijdens de back-up laat geen `.part` achter; "Schijf veilig verwijderen" na een geslaagde back-up werkt. Het venster sluit niet tijdens een back-up.
- [ ] **T11.5** Tandwiel: verkeerd wachtwoord geeft een melding; het wachtwoordveld toont bolletjes en kopiëren werkt niet; na `operatorTimeoutMinutes` zonder activiteit sluit het operatorscherm.
- [ ] **T11.6** Bronmap toevoegen via de mapkiezer, tag, verwijderen; een instelling wijzigen; daarna is `usb-backup.json` ondertekend (`usb-backup-cli run --dry-run` geeft geen exitcode 7).
- [ ] **T11.7** Schijven: drie schijven registreren (naam wordt voorgesteld) en vergeten met echte schijven; de lijst toont de laatste back-up en wie aan de beurt is.
- [ ] **Eerste installatie** Op een pc zonder sleutels: het tandwiel vraagt geen wachtwoord maar start de eerste installatie (als administrator); zonder beheerdersrechten verschijnt de uitleg daarover. Een bestaand bestand voor de break-glass-sleutel wordt geweigerd en blijft staan. Na de bevestiging gaat het instellingenscherm open en reageert het hoofdvenster daarna nog. Een tweede klik op het tandwiel vraagt het wachtwoord. Met een andere Windows-gebruiker (zonder vault) verschijnt de uitleg en worden geen nieuwe sleutels aangeboden.
- [ ] **Licentie** De setup toont de licentiepagina; menu **Help → About** (zonder wachtwoord) en **About** in de instellingen tonen versie, platform, instellingenmap, sleutelstatus en licentie. **File → Settings** doet hetzelfde als het tandwiel; Settings en Exit zijn grijs tijdens een back-up.
- [ ] **Terugzetten annuleren** "Save in…" en dan Cancel zet niets terug (ook niet op het bureaublad); "To Restored on the desktop" en een gekozen map werken.
- [ ] **Icoon** in het venster, de taakbalk, Verkenner, het startmenu, Apps en de setup; scherp bij 100%, 150% en 200% schaling.
- [ ] **Om de beurt** Met drie echte schijven: de verkeerde wordt geweigerd met de juiste naam; na een back-up staat "Next time: …" in het venster. Elke frequentie onder Algemeen (bij elk insteken, dag, week, maand) maakt de verwachte map.
- [ ] **Keuzelijsten** Algemeen (hoe vaak) en Logboek (taal) klappen open en alle keuzes zijn te kiezen en worden opgeslagen.
- [ ] **T11.8** Sleutels: wachtwoord tweemaal (min. 12 tekens), bestand kiezen voor de break-glass-sleutel, scherm gaat pas verder na het aanvinken van de bevestiging (sluiten kan dan ook niet), Printen werkt. Wachtwoord wijzigen, sleutel vervangen en schijf bijwerken.
- [ ] **T11.9** Het vinkje "Start a backup when a registered disk is inserted" onder Algemeen toont wat er in Taakplanner staat, en aan- en uitzetten werkt meteen (ook als de taak al weg was).
- [ ] **T11.10** Bladeren: weken en bronmappen links, mappen rechts (dubbelklik, Omhoog, Zoeken), een los bestand terugzetten naar het bureaublad (`Bureaublad\Restored` als standaard) en naar de oorspronkelijke plek; Break-glass testen met het bestand.
- [ ] **T11.11** Logboek openen; taal omzetten naar Nederlands en terug (na herstart van het venster).

## Systeemvak (fase 14, alleen Windows)

Welke schijf nieuw is en wanneer het venster naar voren komt, is met automatische tests op Linux gedekt (`internal/gui/tray_test.go`); het register in de Windows-CI. Pictogram en venster zijn alleen gecompileerd. Te testen op een Windows-pc:

- [ ] **Uit (standaard)** Alles werkt als eerst: het kruisje sluit het programma, er is geen pictogram, `--auto` haalt het venster naar voren, `--tray` stopt meteen.
- [ ] **Aan** Vinkje aanzetten (met beheerdersrechten) en de instellingen sluiten: het pictogram verschijnt zonder herstart. Uitzetten haalt het weg en zet ook starten bij inloggen uit. Zonder beheerdersrechten: een melding, en het vinkje springt terug.
- [ ] **Kruisje** verbergt het venster, ook tijdens een back-up; de eerste keer met een melding. Klik op het pictogram opent het. Alt+F4 doet hetzelfde als het kruisje.
- [ ] **Pictogram** Rechtsklik: openen en afsluiten; het menu sluit bij een klik ernaast. Afsluiten is grijs tijdens een back-up, met een open instellingenvenster en met een open wachtwoordvraag. De tekst bij het pictogram volgt de toestand en het percentage. Scherp bij 100%, 150% en 200%.
- [ ] **Geregistreerde schijf** met verborgen én met geminimaliseerd venster: het venster komt naar voren, zonder de focus over te nemen (typ in Kladblok), en de back-up start. Verbergen tijdens de back-up: het venster blijft weg; na afloop een melding. Bij een waarschuwing of fout komt het terug.
- [ ] **Andere stick** (ook met een geregistreerde schijf er al in, en met de USB-taak aan): er gebeurt niets zichtbaars.
- [ ] **Verkeerde schijf**: het venster zegt welke wel. Dezelfde schijf eruit en binnen twee seconden er weer in: wordt weer opgemerkt.
- [ ] **Twee schijven**: de tweede tijdens de back-up van de eerste, met `schedule` `insert`: krijgt daarna zijn eigen back-up.
- [ ] **Lang aan** Venster gestart door de USB-taak (na het aanzetten van het vinkje, of na opnieuw inloggen: een exemplaar dat al liep houdt misschien de oude limiet), gesloten met het kruisje: na meer dan 12 uur staat het pictogram er nog en start een back-up bij insteken.
- [ ] **Bezig** Schijf insteken tijdens een herstel of controle in het instellingenscherm: de back-up start binnen een minuut nadat dat klaar is.
- [ ] **Twee gebruikers** (snelle gebruikerswisseling), allebei met pictogram: één maakt de back-up, bij de ander komt geen venster.
- [ ] **Verkenner herstarten** (Taakbeheer): het pictogram komt terug.
- [ ] **Starten bij inloggen** Vinkje aan: staat in Taakbeheer > Opstarten; na opnieuw inloggen staat het pictogram er, zonder venster. Met de juiste schijf er al in start de back-up, met `schedule` `insert` niet. Verwijderen van USB Backup haalt het weg. Een gewijzigd `usb-backup.json`: het venster komt bij het inloggen met de melding.
- [ ] **Afmelden en afsluiten** van Windows met verborgen venster wordt niet tegengehouden.

## Installers (fase 12)

- [ ] **T12.1** `USB-Backup-Setup.exe` (gebouwd met `make setup-windows`): installeren naar `C:\Program Files\USB Backup`, startmenu, `usb-backup-cli` in PATH (nieuwe console), vermelding in Apps en onderdelen, `C:\ProgramData\USB Backup` alleen schrijfbaar voor beheerders (`icacls`) en `logs` ook voor gebruikers; na installatie opent het venster. Bijwerken: nieuwe setup over de oude, instellingen en registraties blijven. Verwijderen: exe's en taken weg, configuratie en vault blijven tenzij aangevinkt. Stil: `/S`.
- [ ] **T12.2** Op Debian of Ubuntu: `sudo apt install ./usb-backup_*_amd64.deb` installeert `/usr/bin/usb-backup-cli`, `/etc/usb-backup` (root, 0755), de systemd-units (niet ingeschakeld) en de man-page (`man usb-backup-cli`); er draait niets. `sudo apt remove usb-backup` haalt timers en udev-regel weg; `sudo apt purge usb-backup` ook `/etc/usb-backup`. Het `.tar.gz` met `sudo ./install.sh` en `--remove` werkt op een andere distributie. De CI-stap controleert de inhoud met `dpkg --contents` en `rpm -qlp` (nog niet op GitHub gedraaid).
- [ ] **T12.3** Met een echt code-signing-certificaat in de secrets `WINDOWS_CERT_PFX_BASE64`/`WINDOWS_CERT_PASSWORD`: de CI ondertekent exe's en setup (Eigenschappen > Digitale handtekeningen); zonder certificaat slaagt de stap met een waarschuwing. Lokaal getest met een zelfondertekend certificaat (zonder tijdstempel) en zonder certificaat.
- [ ] **T12.4** Een test-tag (bijv. `v0.1.0-rc1`) op GitHub levert via `release.yml` een release met: Windows-zips (amd64, arm64), `USB-Backup-Setup.exe`, Linux-`.tar.gz`, `.deb` en `.rpm` (amd64, arm64), `SHA256SUMS` met `SHA256SUMS.sig` en `.pem` (cosign). Lokaal gecontroleerd met `goreleaser release --snapshot --skip=publish,sign` (alle bestanden, setup in de checksums); nog niet op GitHub gedraaid.
- [ ] **T12.5** Een nieuwe gebruiker maakt met alleen de README een back-up en zet een bestand terug (Windows met het venster, Linux met de CLI).
