# Handmatige tests

Dingen die alleen met echte hardware, Windows, Taakplanner, udev of de GUI
te controleren zijn. In de bouwomgeving (Linux, zonder Windows-machine en
zonder USB-schijven) zijn deze niet gedaan; ze staan hier als checklist.

## Windows (fase 7)

- [ ] **T7.1** Een echte USB-schijf wordt gevonden: `usb-backup-cli disks` toont schijfletter, label, bestandssysteem, volume-serienummer en vrije ruimte. De systeemschijf ontbreekt.
- [ ] **T7.2** `disks` toont op een echte schijf alle drie de kenmerken: volume-serienummer, partitie-ID (GPT-GUID, of `MBR:<signature>:<nummer>`) en hardware-serienummer. Controleer ook een goedkope stick zonder serienummer (hardware-serienummer leeg).
- [ ] **T7.3** `usb-backup-cli status` toont Healthy, Warning, Unhealthy of Unknown; er verschijnt geen PowerShell-venster.
- [ ] Veilig verwijderen (`ejectAfter: true` of de GUI-knop) werkt; een schijf die nog in gebruik is, geeft een duidelijke fout.

## Linux (fase 7)

- [ ] **T7.4** CI-stap "Loop-device tests" (`sudo go test -tags integration,loopdev ./internal/volume/...`) is groen op GitHub (nog niet gedraaid).
- [ ] Een echte USB-stick die niet gekoppeld is, wordt bij `run` via `udisksctl` gekoppeld en daarna weer ontkoppeld (als gewone gebruiker), en als root naar `/run/usb-backup/<uuid>` met `nosuid,nodev,noexec`.
- [ ] **T7.5** `status` toont de gezondheid via `smartctl -H` op een schijf die SMART doorgeeft; zonder smartmontools "Unknown".
- [ ] **T7.7 (Windows)** `TestLongPaths` slaagt op de Windows-CI-runner (pad langer dan 260 tekens, backup en herstel).

## Planning (fase 8)

- [ ] **T8.2** `usb-backup-cli install` (Windows): de taak "USB Backup" verschijnt in Taakplanner, start bij het insteken van een geregistreerde schijf (met de GUI als `usb-backup.exe` ernaast staat) en toont het venster in de sessie van de gebruiker. `install --trigger daily --at 02:00` maakt "USB Backup (daily)". `uninstall --trigger all` haalt alles weg.
- [ ] **T8.3** Op Linux (als root): `install --trigger daily` maakt `usb-backup-daily.timer` (zichtbaar in `systemctl list-timers`); `install --trigger usb` maakt de udev-regel en `usb-backup@.service`; insteken van een geregistreerde stick start de service (`journalctl -u 'usb-backup@*'`).

## GUI (fase 11, alleen Windows)

De logica van het statusscherm (alle toestanden, teksten, resterende tijd), de time-out van het operatorscherm en de koppeling met de CLI (versiecontrole, JSON-regels, annuleren, wachtwoord via de pipe) zijn met automatische tests op Linux gedekt (`internal/gui/*_test.go`, nep-CLI). De vensters zelf (windigo) zijn alleen gecompileerd. Te testen op een Windows-pc:

- [ ] **T11.1** `usb-backup.exe` opent het venster; een tweede start brengt het bestaande venster naar voren. Met een `usb-backup-cli.exe` van een andere versie verschijnt een duidelijke foutmelding.
- [ ] **T11.2** Elke toestand verschijnt (geen schijf, verkeerde schijf, onbekende schijf, bezig met bronmap en percentage, controleren, gelukt, gelukt met waarschuwing, mislukt, bijwerken door beheerder). Maak per toestand een schermafbeelding in `docs/screenshots/` (nog niet gemaakt).
- [ ] **T11.3** Schijf insteken verandert het scherm binnen enkele seconden en start de back-up; `usb-backup.exe --auto` (USB-trigger) start direct.
- [ ] **T11.4** Annuleren tijdens de back-up laat geen `.part` achter; "Schijf veilig verwijderen" na een geslaagde back-up werkt. Het venster sluit niet tijdens een back-up.
- [ ] **T11.5** Tandwiel: verkeerd wachtwoord geeft een melding; het wachtwoordveld toont bolletjes en kopiëren werkt niet; na `operatorTimeoutMinutes` zonder activiteit sluit het operatorscherm.
- [ ] **T11.6** Bronmap toevoegen via de mapkiezer, tag, verwijderen; een instelling wijzigen; daarna is `usb-backup.json` ondertekend (`usb-backup-cli run --dry-run` geeft geen exitcode 7).
- [ ] **T11.7** Schijven: registreren (naam, oneven/even) en vergeten met een echte schijf.
- [ ] **Eerste installatie** Op een pc zonder sleutels: het tandwiel vraagt geen wachtwoord maar start de eerste installatie (als administrator); zonder beheerdersrechten verschijnt de uitleg daarover. Een bestaand bestand voor de break-glass-sleutel wordt geweigerd en blijft staan. Na de bevestiging gaat het instellingenscherm open en reageert het hoofdvenster daarna nog. Een tweede klik op het tandwiel vraagt het wachtwoord. Met een andere Windows-gebruiker (zonder vault) verschijnt de uitleg en worden geen nieuwe sleutels aangeboden.
- [ ] **Licentie** De setup toont de licentiepagina; menu **Help → About** (zonder wachtwoord) en **About** in de instellingen tonen versie, platform, instellingenmap, sleutelstatus en licentie. **File → Settings** doet hetzelfde als het tandwiel; Settings en Exit zijn grijs tijdens een back-up.
- [ ] **Keuzelijsten** Schijven (oneven/even weken), Planning (geen/dagelijks/wekelijks en dag) en Logboek (taal) klappen open en alle keuzes zijn te kiezen en worden opgeslagen.
- [ ] **T11.8** Sleutels: wachtwoord tweemaal (min. 12 tekens), bestand kiezen voor de break-glass-sleutel, scherm gaat pas verder na het aanvinken van de bevestiging (sluiten kan dan ook niet), Printen werkt. Wachtwoord wijzigen, sleutel vervangen en schijf bijwerken.
- [ ] **T11.9** Planning: USB-trigger aan/uit en een dagelijkse of wekelijkse taak verschijnen in Taakplanner.
- [ ] **T11.10** Bladeren: weken en bronmappen links, mappen rechts (dubbelklik, Omhoog, Zoeken), een los bestand terugzetten naar het bureaublad (`Bureaublad\Restored` als standaard) en naar de oorspronkelijke plek; Break-glass testen met het bestand.
- [ ] **T11.11** Logboek openen; taal omzetten naar Nederlands en terug (na herstart van het venster).

## Installers (fase 12)

- [ ] **T12.1** `USB-Backup-Setup.exe` (gebouwd met `make setup-windows`): installeren naar `C:\Program Files\USB Backup`, startmenu, `usb-backup-cli` in PATH (nieuwe console), vermelding in Apps en onderdelen, `C:\ProgramData\USB Backup` alleen schrijfbaar voor beheerders (`icacls`) en `logs` ook voor gebruikers; na installatie opent het venster. Bijwerken: nieuwe setup over de oude, instellingen en registraties blijven. Verwijderen: exe's en taken weg, configuratie en vault blijven tenzij aangevinkt. Stil: `/S`.
- [ ] **T12.2** Op Debian of Ubuntu: `sudo apt install ./usb-backup_*_amd64.deb` installeert `/usr/bin/usb-backup-cli`, `/etc/usb-backup` (root, 0755), de systemd-units (niet ingeschakeld) en de man-page (`man usb-backup-cli`); er draait niets. `sudo apt remove usb-backup` haalt timers en udev-regel weg; `sudo apt purge usb-backup` ook `/etc/usb-backup`. Het `.tar.gz` met `sudo ./install.sh` en `--remove` werkt op een andere distributie. De CI-stap controleert de inhoud met `dpkg --contents` en `rpm -qlp` (nog niet op GitHub gedraaid).
- [ ] **T12.3** Met een echt code-signing-certificaat in de secrets `WINDOWS_CERT_PFX_BASE64`/`WINDOWS_CERT_PASSWORD`: de CI ondertekent exe's en setup (Eigenschappen > Digitale handtekeningen); zonder certificaat slaagt de stap met een waarschuwing. Lokaal getest met een zelfondertekend certificaat (zonder tijdstempel) en zonder certificaat.
- [ ] **T12.4** Een test-tag (bijv. `v0.1.0-rc1`) op GitHub levert via `release.yml` een release met: Windows-zips (amd64, arm64), `USB-Backup-Setup.exe`, Linux-`.tar.gz`, `.deb` en `.rpm` (amd64, arm64), `SHA256SUMS` met `SHA256SUMS.sig` en `.pem` (cosign). Lokaal gecontroleerd met `goreleaser release --snapshot --skip=publish,sign` (alle bestanden, setup in de checksums); nog niet op GitHub gedraaid.
- [ ] **T12.5** Een nieuwe gebruiker maakt met alleen de README een back-up en zet een bestand terug (Windows met het venster, Linux met de CLI).
