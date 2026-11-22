# USB Backup – ontwerp

Nov 22, 2026 · @Maurice

## Doel en scope

Eén losse `usb-backup.exe` maakt bij het inpluggen van een back-upschijf automatisch een gecontroleerde zip van `C:\Data`. In oneven weken gaat die naar BACKUP1, in even weken naar BACKUP2. Het resultaat verschijnt op het scherm.

Wat erin zit:

- Twee programma's in Go, cross-compiled vanaf Linux: `usb-backup-cli` doet het werk (Windows en Linux), `usb-backup` is de GUI (Windows)
- Start automatisch bij het inpluggen van een geregistreerde schijf, of gepland via Taakplanner of cron
- Meerdere bronmappen met elk een tag; per bronmap een zip in een weekmap (ISO-week, instelbaar)
- Versleuteling met age, post-quantum, met een operationele sleutel in een vault en een break-glass-sleutel
- SHA-256 per bestand en per zip, volledige controle na het schrijven, en een index om te bladeren en losse bestanden te herstellen
- Schijfherkenning met ID-bestand en serienummers, ondertekend door de operator, net als de instellingen
- Rollen: gebruiker (alleen inpluggen), operator (vault-wachtwoord), noodherstel (break-glass)
- Rotatie: standaard 25 mappen per schijf; sleutelrotatie met opnieuw inpakken van bestaande back-ups
- Installers: setup voor Windows, deb, rpm en tar.gz voor Linux

Wat er niet in zit: een systeemimage om Windows terug te zetten, incrementele back-ups en back-ups naar netwerk of cloud. Versleuteling staat bij de open vragen.

## Architectuur

Het programma is in Go 1.24+ geschreven en gebruikt alleen de standaardbibliotheek. De Windows-API wordt via `syscall` aangeroepen (kernel32, user32), zonder cgo. Daardoor levert `GOOS=windows go build` vanaf elk platform één exe op. Met `-H windowsgui` opent er geen consolevenster.

Alles wat Windows-specifiek is, zit in `_windows.go`-bestanden met een stub ernaast (`_other.go`). Zo draait de kernlogica (zip, controle, rotatie) ook in tests op Linux en macOS, met mappen als nep-USB-schijven.

| Pakket | Verantwoordelijkheid | Platform |
| --- | --- | --- |
| `cmd/usb-backup` | Subcommando's (`run`, `verify`, `status`, `install`, `uninstall`, `version`), vlaggen en exitcodes; zonder argument verschijnt de installatievraag. Hieruit komen beide exe's | alle |
| `internal/config` | Standaardwaarden, optioneel `usb-backup.json` in de configuratiemap (Windows C:\\ProgramData\\USB Backup, Linux /etc/usb-backup of \~/.config/usb-backup); vlaggen gaan voor | alle |
| `internal/week` | ISO-week via `time.ISOWeek()`, dan jaar, week en het juiste label | alle |
| `internal/archive` | Zip schrijven (`archive/zip`, zip64), SHA-256 tijdens het schrijven, manifest | alle |
| `internal/verify` | Zip opnieuw openen, alles uitpakken naar `io.Discard`, CRC-32 en SHA-256 vergelijken; gebruikt door `run` en `verify` | alle |
| `internal/rotate` | Weekmappen vinden, sorteren, de oudste verwijderen | alle |
| `internal/lock` | Er draait maar één back-up tegelijk (benoemde mutex `Global\UsbBackup`) | windows + stub |
| `internal/volume` | Schijven op label zoeken; type, vrije ruimte, bestandssysteem, gezondheid | windows + stub |
| `internal/ui` | Meldingen via `MessageBoxW`, of tekst en JSON naar stdout | windows + stub |
| `internal/task` | Taken registreren (USB-, dagelijkse en wekelijkse trigger) met `schtasks` en een XML-definitie | windows + stub |
| `internal/logx` | Logboek in de configuratiemap, plus een rapport per back-up | alle |

## Back-upflow

Bij elke aansluiting doorloopt `usb-backup.exe run` deze stappen. De schijfkeuze volgt het ISO-weeknummer: oneven = BACKUP1, even = BACKUP2.

&#91;embedded content: back-upflow · 10 stappen, 5 uitwegen\]

Geen enkele uitweg aan de rechterkant verwijdert iets. Oude weekmappen gaan pas weg als de nieuwe zip volledig is uitgepakt en elke hash klopt.

## Indeling op de USB-schijf

Elke week krijgt een eigen map met één zip en de hashbestanden die erbij horen. Een zip heet pas `data.zip` als de controle geslaagd is.

```
X:\
└─ Backup\
   ├─ disk-id.json            ID van deze schijf (zie Schijfherkenning)
   ├─ laatste-backup.txt      tijdstip van de laatste geslaagde back-up
   ├─ 2026\
   │  ├─ 47\
   │  │  ├─ Data.zip.age             één zip per bronmap, genoemd naar zijn tag
   │  │  ├─ Data.zip.age.sha256
   │  │  ├─ Boekhouding.zip.age
   │  │  ├─ Boekhouding.zip.age.sha256
   │  │  └─ rapport.txt              per tag: tijden, aantallen, status
   │  └─ 49\ …
   └─ 2027\ …
```

Elke bronmap krijgt een korte tag (letters, cijfers, - en \_), en die tag wordt de bestandsnaam. Een weekmap telt pas als geslaagd als alle zips daarin gecontroleerd zijn. Mislukt één bronmap, dan blijven de andere zips staan, wordt de week als mislukt gemeld en wordt er niets geroteerd.

- **Zip:** Deflate, met zip64 automatisch boven 4 GB of 65.535 bestanden. Bestanden die al gecomprimeerd zijn (jpg, png, mp4, mov, zip, 7z, rar, docx, xlsx, pptx) worden opgeslagen zonder compressie (Store). Dat scheelt veel tijd.
- **Inhoud:** paden met `/` en de UTF-8-vlag, zodat namen met accenten goed blijven. Lege mappen komen erin als map-entries. De wijzigingsdatum van elk bestand blijft bewaard.
- **Manifest in de zip:** als laatste entry komt `_backup/manifest.sha256` erin. Zo blijft de zip ook op zichzelf controleerbaar als hij ergens anders naartoe gekopieerd wordt.
- **Tijdelijke naam:** er wordt eerst naar `data.zip.part` geschreven, en dat wordt pas na een geslaagde controle hernoemd. Een tweede back-up in dezelfde week vervangt de oude zip dus pas als de nieuwe goed is.
- **Hashformaat:** `<sha256>  <pad>`, compatibel met `sha256sum -c`. Handmatig controleren kan op Windows met `certutil -hashfile data.zip SHA256` of `Get-FileHash`.

## Schijfherkenning

Een schijf wordt herkend aan een ID-bestand op de schijf, samen met hardwarekenmerken die op de pc geregistreerd staan. Het label is alleen nog een naam voor mensen. Dit is belangrijk omdat de rotatie mappen verwijdert: dat mag nooit gebeuren op een willekeurige stick die toevallig BACKUP1 heet.

### Kenmerken

| Kenmerk | Uitgelezen met | Verandert bij | Rol |
| --- | --- | --- | --- |
| ID-bestand `Backup\disk-id.json` (willekeurige UUID) | Bestand lezen | Verwijderen of formatteren | Hoofdsleutel |
| Volume-serienummer | `GetVolumeInformationW` | Formatteren | Moet kloppen |
| Partitie-GUID (GPT) of disk signature (MBR) | `IOCTL_DISK_GET_PARTITION_INFO_EX` | Opnieuw partitioneren | Moet kloppen |
| Hardware-serienummer van de USB-schijf | `IOCTL_STORAGE_QUERY_PROPERTY` | Ander apparaat | Moet kloppen als het er is. Goedkope sticks hebben soms geen of geen uniek serienummer |
| Label | `GetVolumeInformationW` | Hernoemen | Alleen weergave |

### Registreren

`usb-backup-cli enroll E: --name BACKUP1 --weeks odd` doet drie dingen:

1. Het schrijft `Backup\disk-id.json` met een nieuwe UUID, als verborgen en alleen-lezen bestand.
2. Het leest de serienummers en de partitie-ID uit.
3. Het slaat alles op in `usb-backup.disks.json` in de configuratiemap.

```
{
  "name": "BACKUP1",
  "weeks": "odd",
  "id": "3f9c2b1e-7a4d-4e8b-9c21-5d6e7f8a9b0c",
  "volumeSerial": "A41C-77E2",
  "partitionId": "{6b1f0c3e-2d4a-4f5b-8e9c-1a2b3c4d5e6f}",
  "hardwareSerial": "575834314131383436",
  "enrolled": "2026-11-22"
}
```

Verder zijn er `usb-backup-cli disks`, dat geregistreerde en aangesloten schijven naast elkaar toont, en `usb-backup-cli forget BACKUP1`. De koppeling van oneven en even weken komt zo uit de registratie, niet uit het label.

### Ondertekening door de operator

Registraties en instellingen worden ondertekend met een apart ondertekeningssleutelpaar van de operator. Zo kan een gewone gebruiker geen schijf toevoegen en geen instellingen veranderen, ook niet door bestanden op de pc aan te passen.

| Sleutelpaar | Doel | Privésleutel | Publieke sleutel |
| --- | --- | --- | --- |
| Versleuteling (age, post-quantum) | Back-ups versleutelen en ontsleutelen | Vault (operationeel) en kluis (break-glass) | `usb-backup.recipients` |
| Ondertekening (Ed25519) | Registraties, instellingen en ID-bestanden ondertekenen | Vault, naast de operationele sleutel | `usb-backup.signing.pub` in de configuratiemap |

- **Wat wordt ondertekend:** `usb-backup.disks.json` (de registratielijst), `usb-backup.json` (de instellingen, inclusief bronmappen en bewaartermijn), en elk `disk-id.json` op een USB-schijf. Het ID-bestand bevat dan naast de UUID ook de serienummers van die schijf, met de handtekening erover.
- **Bij elke run:** eerst worden de handtekeningen van instellingen en registratielijst gecontroleerd met de publieke sleutel. Klopt er een niet, dan stopt het programma met een melding "instellingen zijn gewijzigd zonder beheerder" en exitcode 7. Daarna wordt de schijf gecontroleerd: handtekening van het ID-bestand, UUID in de lijst, serienummers gelijk.
- **Ondertekenen gebeurt alleen** in het operatorscherm of met `enroll`, `forget` en `config set`, allemaal na het vault-wachtwoord.
- **Ed25519 en niet post-quantum:** een handtekening wordt bij elke run vers gecontroleerd en kan ook vervangen worden door opnieuw te ondertekenen. Een quantumcomputer die later komt, kan oude handtekeningen dus niet misbruiken voor back-ups die al bestaan. Overstappen naar ML-DSA kan later, zodra dat in de Go-standaardbibliotheek zit.
- **Grens:** wie adminrechten heeft op de pc, kan ook de publieke sleutel vervangen en alles opnieuw ondertekenen. De configuratiemap krijgt daarom rechten zodat alleen beheerders erin mogen schrijven (op Linux eigenaar root, modus 0644). De ondertekening beschermt tegen gewone gebruikers en per ongeluk gewijzigde bestanden, niet tegen een beheerder met kwade bedoelingen.
- **Break-glass:** met de break-glass-sleutel kan een nieuwe vault met een nieuw ondertekeningssleutelpaar worden gemaakt. Daarna moeten de schijven opnieuw worden geregistreerd.

### Controle bij elke run

- **Toegestaan:** alleen als het ID-bestand er is en de UUID bekend is, volume-serienummer en partitie-ID kloppen, en het hardware-serienummer klopt (als dat bij registratie is vastgelegd).
- **ID-bestand gekopieerd naar een andere schijf:** de serienummers kloppen niet. Bovendien klopt de handtekening dan niet meer, omdat de serienummers in het ondertekende ID-bestand staan. Er volgt een melding "onbekende schijf" en exitcode 6. Er wordt niets geschreven of verwijderd.
- **Schijf geformatteerd:** het ID-bestand is weg, dus de schijf is onbekend. Opnieuw `enroll` uitvoeren.
- **Gewone USB-stick zonder ID-bestand:** wordt stil genegeerd, zonder melding.

`disk-id.json` bevat geen geheimen. Wie de schijf heeft, kan het bestand lezen, maar zonder de registratie op de pc heeft hij daar niets aan.

## Controle, schijfstatus, rotatie en meldingen

Een back-up telt alleen als geslaagd als de zip volledig uitgepakt is en elke hash klopt. Pas daarna worden oude weekmappen verwijderd.

### Controle van de zip

1. Tijdens het schrijven wordt de SHA-256 per bestand berekend over precies de bytes die de zip ingaan (`io.MultiWriter`).
2. Daarna gaat de zip met `Sync()` naar de schijf en wordt hij gesloten.
3. De zip wordt opnieuw geopend met `archive/zip`. Elk bestand wordt volledig uitgepakt naar `io.Discard`; Go controleert daarbij de CRC-32. Daarna wordt de SHA-256 vergeleken met het manifest, en het aantal bestanden moet precies kloppen.
4. De hele zip wordt opnieuw gelezen voor `data.zip.sha256`.
5. Alleen als alles klopt, wordt `data.zip.part` hernoemd naar `data.zip`.

Uitpakken naar `io.Discard` bewijst dat de zip uitpakbaar is, zonder extra schijfruimte te gebruiken. Er is wel één beperking: Windows kan het teruglezen uit het cachegeheugen halen in plaats van van de schijf (zie open vragen).

### Schijfstatus

| Controle | Hoe | Bij een probleem |
| --- | --- | --- |
| Juiste schijf voor deze week | Geregistreerde schijf (zie Schijfherkenning) tegen weeknummer: oneven = BACKUP1, even = BACKUP2 | Waarschuwing, geen back-up |
| Bestandssysteem | `GetVolumeInformationW` | FAT32 en een zip van meer dan 4 GB verwacht: fout, advies om exFAT of NTFS te gebruiken |
| Vrije ruimte | `GetDiskFreeSpaceExW`; nodig = grootte van `C:\Data` + 5% + 100 MB, opgeteld over alle bronmappen | Fout, er wordt niets verwijderd |
| Schrijfbaar | Testbestand aanmaken en verwijderen | Fout |
| Gezondheid | `HealthStatus` van `MSFT_Disk` (zoals bij `Get-Disk`), via verborgen PowerShell of WMI | Warning: back-up gaat door, gele melding. Unhealthy: back-up gaat door, rode melding "vervang deze schijf". Onbekend: alleen vermelden |

Veel USB-sticks geven geen SMART-gegevens door. Daar is "Healthy" dus alleen wat Windows meldt, geen garantie.

### Rotatie

Na een geslaagde controle worden de mappen `jjjj\ww` gesorteerd. De oudste worden verwijderd tot er 25 over zijn, en een lege jaarmap gaat ook weg. Alleen mappen die precies aan dat patroon voldoen, worden aangeraakt. Elke schijf roteert apart, dus samen bewaren ze ongeveer 50 weken geschiedenis.

### Meldingen

De taak draait als de ingelogde gebruiker, zodat een `MessageBoxW` (bovenop, met focus) zichtbaar is. Daar is geen extra bibliotheek voor nodig.

| Situatie | Icoon | Inhoud |
| --- | --- | --- |
| Start | info, sluit vanzelf als het programma klaar is | "Back-up naar BACKUP1 bezig, schijf niet verwijderen" |
| Geslaagd | info | Schijf en letter, week, aantal bestanden, grootte bron en zip, "controle OK", gezondheid, vrije ruimte, aantal weekmappen, "je kunt de schijf verwijderen" |
| Geslaagd, bestanden overgeslagen | waarschuwing | Als hierboven, plus de eerste 10 vergrendelde bestanden |
| Verkeerde schijf | waarschuwing | "Week 47 is voor BACKUP1, maar BACKUP2 is aangesloten" |
| Mislukt | fout | Reden, en "oude back-ups zijn niet verwijderd" |

## Installatie, configuratie en logging

Na de setup (zie Installatiepakketten) registreert het programma zelf de taak in Taakplanner; dat kan ook later vanuit het instellingenscherm of met usb-backup-cli install.

### Installatie

- **Dubbelklikken zonder argument:** de vraag "Automatische USB-back-up instellen?" verschijnt, met bron en schijfindeling erbij. Bij "Ja" wordt de taak geregistreerd.
- **Taak "USB Backup":**
  - Trigger: gebeurtenis 1006 in `Microsoft-Windows-Partition/Diagnostic`
  - Actie: `"<pad>\usb-backup.exe" run`
  - Draait als de huidige gebruiker, alleen als die is ingelogd (InteractiveToken, zodat meldingen zichtbaar zijn), zonder adminrechten (LeastPrivilege)
  - MultipleInstances = IgnoreNew; geen beperking op batterij; maximaal 12 uur
- **Registratie:** de XML-definitie wordt als UTF-16 LE met BOM naar een tijdelijk bestand geschreven en geregistreerd met `schtasks /Create /TN "USB Backup" /XML <bestand> /F`, in een verborgen venster.
- **Verwijderen:** `usb-backup.exe uninstall` verwijdert de taak. Het pad naar de exe staat in de taak, dus wie de exe verplaatst, moet opnieuw installeren.

### Configuratie

In de configuratiemap (Windows C:\\ProgramData\\USB Backup, Linux /etc/usb-backup of \~/.config/usb-backup) mag een `usb-backup.json` staan. Ontbreekt die, dan gelden de standaardwaarden.

Alle instellingen staan in het operatorscherm en zijn ook met `usb-backup-cli config set <sleutel> <waarde>` te wijzigen. Elke wijziging wordt gecontroleerd voordat hij wordt opgeslagen, en gelogd met oude en nieuwe waarde.

| Groep | Sleutel | Standaard | Betekenis |
| --- | --- | --- | --- |
| Bronnen | `sources` | `[{"tag":"Data","path":"C:\\Data"}]` | Bronmappen, elk met tag, pad en eigen `exclude` |
| Bronnen | `exclude` | `*.tmp`, `~$*`, `Thumbs.db`, `desktop.ini` | Uitsluitingen voor alle bronmappen |
| Bronnen | `followSymlinks` | `false` | Linux: symlinks volgen of als link opslaan |
| Doel | `targetFolder` | `Backup` | Map op de USB-schijf waarin alles komt, bijvoorbeeld `Back-ups\Kantoor` |
| Doel | `folderPattern` | `{jaar}\{week}` | Indeling van de weekmappen; ook `{jaar}-{week}` of `{jaar}\{maand}\{dag}` bij een dagschema |
| Doel | `archiveName` | `{tag}.zip` | Bestandsnaam per bronmap |
| Schema | `schedule` | `isoWeek` | Eenheid per map: `isoWeek`, `day` of `month` |
| Schema | `rotation` | `oddEven` | Schijfwissel: `oddEven` (week), `roundRobin` over N schijven, of `any` |
| Schema | `minIntervalHours` | `20` | Binnen dezelfde map pas opnieuw na deze tijd |
| Schema | `duplicateGuardMinutes` | `10` | Een nieuwe trigger binnen deze tijd wordt genegeerd |
| Bewaren | `keepPerDisk` | `25` | Aantal mappen per schijf; `0` = nooit verwijderen |
| Bewaren | `keepMinDays` | `0` | Mappen jonger dan dit nooit verwijderen, ook als `keepPerDisk` is bereikt |
| Bewaren | `freeSpaceAction` | `stop` | Bij te weinig ruimte: `stop`, of `deleteOldest` (oudste map eerst weg, nooit onder `keepMinDays`) |
| Zip | `compressionLevel` | `6` | Deflate-niveau 1 tot 9; `0` = alles zonder compressie |
| Zip | `storeExtensions` | jpg, png, mp4, mov, zip, 7z, rar, docx, xlsx, pptx | Deze types gaan zonder compressie in de zip |
| Zip | `lockedFiles` | `skip` | Vergrendelde bestanden: `skip` met waarschuwing, of `fail` |
| Controle | `verifyAfterBackup` | `true` | Zip na schrijven terugcontroleren |
| Controle | `verifyPreviousOnInsert` | `false` | Bij inpluggen ook de vorige back-up op die schijf terugcontroleren (bitrot) |
| Controle | `minFreePercent` | `5` | Marge boven de geschatte grootte |
| Schijven | `disksFile` | `usb-backup.disks.json` | Geregistreerde schijven, met naam en rol (zie Schijfherkenning) |
| Schijven | `requireHardwareSerial` | `false` | Schijven zonder hardware-serienummer weigeren |
| Schijven | `waitSeconds` | `30` | Hoe lang er op een schijfletter of koppeling gewacht wordt |
| Schijven | `ejectAfter` | `false` | Schijf na een geslaagde back-up automatisch veilig verwijderen |
| Versleuteling | `encryption` | `age` als er ontvangers zijn | `age` of `none` |
| Versleuteling | `recipientsFile` | `usb-backup.recipients` | Publieke sleutels, één per regel |
| Versleuteling | `vaultPath` | `%APPDATA%\USB Backup\herstel.vault` | Plek van de vault, ook op een sleutel-USB |
| Versleuteling | `operatorTimeoutMinutes` | `5` | Operatorscherm en wachtwoord vergeten na deze tijd |
| Meldingen | `notify` | `always` | `always`, `problemsOnly` of `never` |
| Meldingen | `language` | `nl` | Taal van scherm en meldingen: `nl` of `en` |
| Meldingen | `webhookUrl` | leeg | Optioneel: rapport als JSON naar een URL sturen, bijvoorbeeld voor monitoring |
| Logboek | `logPath` | configuratiemap | Map voor `usb-backup.log` |
| Logboek | `logMaxSizeMB` | `1` | Daarboven wordt het logboek omgewisseld |
| Logboek | `logKeep` | `5` | Aantal oude logbestanden |
| Logboek | `logLevel` | `info` | `error`, `warn`, `info` of `debug` |

Wijzigingen in `targetFolder` of `folderPattern` gelden voor nieuwe back-ups. Bestaande mappen blijven staan, en de rotatie herkent alleen mappen volgens het patroon dat in `Backup\keys.json` op de schijf is vastgelegd; zo verwijdert een gewijzigd patroon nooit per ongeluk andere mappen. `config export` en `config import` maken het mogelijk dezelfde instellingen op meerdere pc's te gebruiken (zonder sleutels of vault).

### Logging

- **`usb-backup.log`** staat in de configuratiemap; het instellingenscherm heeft een knop om het te openen: één regel per gebeurtenis, met tijdstempel. Boven 1 MB wordt het bestand hernoemd naar `usb-backup.log.1`.
- **`rapport.txt`** staat in elke weekmap, met de samenvatting van die back-up.

## Grafische interface

Het programma krijgt een eigen venster met twee delen: een statusscherm dat gebruikers zonder uitleg kunnen volgen, en een instellingenscherm voor de beheerder. Het statusscherm opent bij dubbelklikken en kijkt meteen of de juiste schijf aanwezig is.

### Statusscherm (gebruiker)

Een groot icoon, één zin in grote letters en hooguit één knop. Het scherm volgt deze toestanden:

| Toestand | Wat de gebruiker ziet | Knop |
| --- | --- | --- |
| Geen schijf | "Plaats USB-schijf BACKUP1" (of BACKUP2, volgens de week) | Geen; het scherm wacht en reageert vanzelf op inpluggen |
| Verkeerde schijf | "Deze week is BACKUP1 aan de beurt. Haal BACKUP2 eruit en plaats BACKUP1" | Geen |
| Onbekende schijf | "Deze schijf is niet geregistreerd" | Geen |
| Bezig | "Back-up bezig, schijf niet verwijderen", voortgang per bronmap ("Boekhouding, 2 van 3") met percentage en resterende tijd | Annuleren |
| Controleren | "Back-up wordt gecontroleerd", voortgangsbalk | Geen |
| Gelukt | Groen vinkje: "Alles is in orde. Je kunt de schijf verwijderen" | Schijf veilig verwijderen |
| Gelukt met waarschuwing | Oranje: "Back-up gemaakt, maar 3 bestanden waren in gebruik" | Details |
| Mislukt | Rood: "Back-up mislukt" en de reden in gewone taal | Opnieuw proberen |

Onder de hoofdzin staat altijd klein: laatste geslaagde back-up per schijf, gezondheid van de aangesloten schijf en vrije ruimte. Wordt een back-up gestart door de USB-trigger, dan opent hetzelfde venster vanzelf. De losse MessageBox-meldingen vervallen.

### Instellingenscherm (beheerder)

Bereikbaar via een tandwiel rechtsboven, alleen voor de operator en beveiligd met het vault-wachtwoord: wie dat wachtwoord kent, is operator. Het operatorscherm sluit na 5 minuten zonder activiteit. Tabbladen:

- **Algemeen:** lijst met bronmappen, elk met een tag; toevoegen via een mapkiezer, tag aanpassen, verwijderen, en per map uitsluitingen (bijvoorbeeld \*.tmp). Verder het aantal te bewaren weken en de wachttijden
- **Schijven:** aangesloten en geregistreerde schijven naast elkaar; knoppen Registreren (met naam en oneven of even weken) en Vergeten
- **Versleuteling:** aan of uit, sleutelpaar maken met een scherm dat dwingt de privésleutel op te slaan of te printen, ontvangers beheren
- **Planning:** USB-trigger aan of uit, dagelijkse of wekelijkse taak met tijd
- **Controle en herstel:** back-ups controleren of terugzetten met de privésleutel, logboek bekijken

Alles wat hier kan, kan ook met de CLI; het scherm roept dezelfde interne functies aan.

### Rollen

| Rol | Wie | Kan | Heeft nodig |
| --- | --- | --- | --- |
| Gebruiker | Iedereen achter de pc | Schijf insteken, status en voortgang zien, schijf veilig verwijderen | Niets |
| Operator | Beheerder | Instellingen, schijven registreren, sleutels beheren, bladeren, herstellen, rotatie bijwerken, logboek | Vault-wachtwoord |
| Noodherstel | Wie de break-glass-sleutel uit de kluis haalt | Herstellen, en een nieuwe vault aanmaken als het wachtwoord kwijt is | Break-glass-sleutel |

Voor back-ups is geen geheim nodig, omdat versleutelen alleen de publieke sleutels gebruikt. Daardoor werken automatische back-ups zonder dat er een wachtwoord op de pc staat. Op de commandline geldt hetzelfde: `run`, `status` en `disks` zonder wachtwoord; `list`, `restore`, `verify --full`, `keys` en `enroll` vragen het vault-wachtwoord.

### Techniek

- **Bibliotheek:** windigo, een Go-wrapper rond de Win32-API zonder cgo. Cross-compilen blijft daardoor één `go build`, en het venster ziet eruit als een gewoon Windows-programma. Fyne en Wails vallen af: Fyne vraagt cgo en OpenGL, Wails een webview en een zwaardere build.
- **Een instantie:** opent iemand het programma terwijl een back-up loopt, dan komt het bestaande venster naar voren.
- **Inpluggen detecteren:** het venster luistert naar `WM_DEVICECHANGE`, zodat het meteen reageert zonder steeds te zoeken.
- **Achtergrond:** de back-up draait in een aparte goroutine, die voortgang naar het venster stuurt. Het venster blijft daardoor bruikbaar.

## Bladeren en herstellen

In het venster kun je door alle back-ups op de aangesloten schijf bladeren en losse bestanden of mappen terugzetten. Omdat de back-ups versleuteld zijn, vraagt het herstelscherm eerst om de privésleutel; die wordt alleen in het geheugen gehouden en nooit opgeslagen.

### Zo werkt het voor de gebruiker

1. **De operator kiest Bestand terugzetten via het tandwiel; gewone gebruikers zien deze knop niet**.
2. Wachtwoord van de vault invullen. In nood kan in plaats daarvan de break-glass-sleutel gekozen, geplakt of als QR-code ingescand worden.
3. Links een lijst met weken ("week 47, 2026") en per week de bronmappen; rechts een mappenboom met naam, grootte en wijzigingsdatum, plus een zoekveld.
4. Bestanden of mappen aanvinken en kiezen: **Terugzetten naar oorspronkelijke plek** (bestaande bestanden krijgen een andere naam, nooit overschrijven zonder vraag) of **Opslaan in…** een gekozen map, standaard `Bureaublad\Hersteld`.
5. Na afloop: aantal teruggezette bestanden en een knop om de map te openen.

### Index per bronmap

Om te bladeren zonder een hele zip van gigabytes te ontsleutelen, schrijft de back-up per bronmap een kleine index: `<tag>.index.age`. Die bevat per bestand het pad, de grootte, de wijzigingsdatum, de SHA-256 en de positie in de zip. Ook de index is versleuteld met age, want bestandsnamen kunnen gevoelig zijn.

- **Bladeren:** alleen de index wordt ontsleuteld; dat duurt een seconde, ook bij grote back-ups.
- **Een bestand terugzetten:** de zip wordt als stroom ontsleuteld tot het gevraagde bestand en daar gestopt. Een bestand achteraan een grote zip kost dus even; de voortgangsbalk toont dat.
- **Controle:** elk teruggezet bestand wordt vergeleken met de SHA-256 uit de index.
- **Index kwijt of beschadigd:** dan valt het scherm terug op de zip zelf door alles te ontsleutelen, en meldt dat dit langer duurt.

Zonder versleuteling werkt alles hetzelfde, maar dan zonder sleutelvraag en met directe toegang tot de zip.

## Versleuteling

De zip wordt versleuteld met [age](https://age-encryption.org) en een publieke sleutel. Op de pc staat daardoor geen wachtwoord of geheime sleutel, en een verloren of gestolen USB-schijf is onleesbaar, ook de bestandsnamen.

### Afweging

| Optie | Geheim op de pc nodig | Bestandsnamen verborgen | Volledige uitpakcontrole automatisch | Terugzetten | Werk |
| --- | --- | --- | --- | --- | --- |
| **age, publieke sleutel (gekozen)** | Nee, alleen de publieke sleutel | Ja | Nee, alleen controle van het versleutelde bestand (zie hieronder) | Eigen `restore`-commando of de `age`-tool, met de privésleutel | Go-bibliotheek `filippo.io/age`, streaming |
| age met wachtwoord, opgeslagen met DPAPI | Ja, beveiligd met het Windows-account | Ja | Ja | Wachtwoord | Iets meer: DPAPI en het beheer van het wachtwoord |
| AES-256-zip (WinZip AE-2) | Ja, het wachtwoord | Nee, namen en groottes blijven zichtbaar | Ja | 7-Zip, met het wachtwoord | Externe zipbibliotheek; ZipCrypto moet vermeden worden |
| BitLocker To Go op de schijven | Nee, automatisch ontgrendelen op deze pc | Ja, de hele schijf | Ja, er verandert niets | Herstelsleutel op een andere pc | Geen code; aanzetten vraagt Windows Pro of hoger |

BitLocker To Go is een goede aanvulling, los van de app: daarmee is de hele schijf versleuteld, ook logboeken en mapnamen. In de app zelf kiezen we age, omdat het niet afhangt van de Windows-editie en er geen geheim op de pc hoeft te staan.

### Sleutelbeheer: operationeel en break-glass

Elke back-up heeft twee ontvangers: een operationele sleutel voor dagelijks herstel en een break-glass-sleutel voor noodgevallen. age werkt al met enveloppen: per bestand een willekeurige sleutel (de file key), die per ontvanger apart wordt ingepakt in de header. Een eigen AES-laag is daarom niet nodig; twee ontvangers geven precies dat model.

| Sleutel | Waar | Beveiliging | Gebruik |
| --- | --- | --- | --- |
| Operationeel | `herstel.vault` op de pc of een USB-sleutel | Privésleutel versleuteld met een wachtwoord (age scrypt) | Herstelscherm: wachtwoord invullen |
| Break-glass | Offline: geprint in een kluis, en eventueel een tweede USB-sleutel | Fysiek | Vault of wachtwoord kwijt, of de pc is weg |

- **Aanmaken:** in het instellingenscherm maakt **Sleutels instellen** beide post-quantum sleutelparen in één stap. De gebruiker kiest een wachtwoord voor de vault (minimaal 12 tekens, tweemaal invoeren). Daarna toont het programma de break-glass-sleutel één keer, met Printen (als tekst plus QR-codes) en Opslaan als. Het gaat pas verder als bevestigd is dat de sleutel bewaard is. Beide publieke sleutels gaan naar `usb-backup.recipients`.
- **Vault zonder de app:** `herstel.vault` is een gewoon age-bestand met wachtwoord. Met de losse tool werkt het ook: `age -d -i herstel.vault data.zip.age` vraagt om het wachtwoord.
- **Wachtwoord wijzigen:** alleen de vault wordt opnieuw versleuteld; de back-ups blijven gelijk.
- **Sleutel vervangen (bijvoorbeeld na een gelekt wachtwoord):** nieuw operationeel sleutelpaar, nieuwe ontvangerslijst. Nieuwe back-ups gebruiken de nieuwe sleutel; bestaande back-ups worden per schijf opnieuw ingepakt (zie Sleutelrotatie).
- **Controle:** `status` meldt het als er nog nooit met de break-glass-sleutel is getest; het herstelscherm heeft een knop **Break-glass testen**.

### Sleutelrotatie

Na `keys rotate` worden alle bestaande back-ups opnieuw ingepakt voor de nieuwe sleutel, schijf voor schijf zodra die wordt ingestoken. Zonder dat zou de oude sleutel tot 50 weken bruikbaar blijven (25 weken op elk van de twee schijven).

- **Opnieuw inpakken, niet opnieuw versleutelen:** een age-bestand begint met een kleine header met de bestandssleutel, ingepakt per ontvanger. Bij rotatie wordt die bestandssleutel met de oude sleutel uitgepakt, opnieuw ingepakt voor de nieuwe ontvangers (operationeel nieuw + break-glass), en de header-MAC opnieuw berekend. De versleutelde inhoud wordt ongewijzigd gekopieerd. Dat gaat zo snel als de schijf kan kopiëren, zonder ontsleutelen of opnieuw zippen.
- **Veilig vervangen:** het nieuwe bestand wordt eerst als `.part` geschreven, met de nieuwe sleutel gecontroleerd (header openen en de hash van de inhoud vergelijken), en pas dan over het oude heen hernoemd. Ook de index-bestanden worden zo omgezet.
- **Eigen code:** filippo.io/age heeft geen functie om alleen de header te vervangen. De header wordt daarom met eigen code gelezen en geschreven volgens de openbare age-specificatie, met tests tegen de bibliotheek.
- **Beperking:** de bestandssleutel blijft dezelfde. Had iemand met de oude sleutel al een kopie van een back-up, dan helpt rotatie voor die kopie niet. Wie dat geval wil afdekken, gebruikt `keys rotate --full`: alles wordt opnieuw versleuteld, wat zo lang duurt als een volledige back-up.

### Sleutelgeschiedenis in de vault

De vault bevat niet één sleutel maar een lijst:

| Veld | Voorbeeld | Betekenis |
| --- | --- | --- |
| `id` | `k2` | Korte naam |
| `fingerprint` | SHA-256 van de publieke sleutel, eerste 16 tekens | Om back-ups aan een sleutel te koppelen |
| `status` | `actief` of `uitgefaseerd` | Alleen de actieve sleutel wordt gebruikt voor nieuwe back-ups |
| `retired` | 2026-12-01 | Wanneer uitgefaseerd |
| `pendingDisks` | `["BACKUP2"]` | Schijven die nog oude back-ups hebben |

age-headers zeggen niet voor welke sleutel ze zijn ingepakt. Daarom schrijft elke back-up de fingerprint van de gebruikte sleutel in `rapport.txt` en in `Backup\keys.json` op de schijf. Een fingerprint van een publieke sleutel is geen geheim.

### Wat de gebruiker ziet

1. Er zit een schijf in met back-ups van een uitgefaseerde sleutel. De gebruiker ziet alleen: "Deze schijf moet door de beheerder worden bijgewerkt"; de back-up zelf gaat gewoon door. In het operatorscherm staat de schijf met de knop **Nu bijwerken**.
2. Bijwerken vraagt het vault-wachtwoord, want de oude privésleutel is nodig. Een automatische back-up loopt gewoon eerst door; het bijwerken blijft dan als melding staan.
3. Na afloop haalt het programma de schijf van `pendingDisks` af. Zijn alle geregistreerde schijven bijgewerkt, dan vraagt het: "De oude sleutel is nergens meer nodig. Definitief verwijderen?" Pas na bevestiging wordt hij uit de vault gewist.

Op de commandline: `usb-backup-cli keys rewrap --target E: --password-stdin` en `usb-backup-cli keys status`. Lukt het opnieuw inpakken niet, dan blijft het oude bestand staan en volgt exitcode 13.

### Werking

- **Sleutels:** `usb-backup-cli keys init` maakt een hybride post-quantum sleutelpaar (ML-KEM-768 + X25519, vereist filippo.io/age v1.3.0 of nieuwer). De publieke sleutel (`age1…`) komt in `usb-backup.recipients` in de configuratiemap. De privésleutels worden beheerd zoals beschreven bij Sleutelbeheer: de operationele in de met wachtwoord beveiligde vault, de break-glass offline. Dat zijn twee ontvangers; er mogen er meer zijn. Ze moeten dan allemaal post-quantum zijn: age weigert een mix met gewone sleutels. Een post-quantum publieke sleutel begint met age1pq1 en is ongeveer 2000 tekens lang, dus hij hoort in het bestand, niet op de commandline.
- **Schrijven:** de stappen zijn bestanden, dan zip, dan age, dan de schijf, in één stroom. Er komt dus nooit een onversleutelde zip op de USB-schijf. Het bestand heet `data.zip.age`.
- **Wat onversleuteld blijft:** alleen `data.zip.age.sha256` en een `rapport.txt` zonder bestandsnamen. `manifest.sha256` gaat alleen mee binnen de versleutelde zip, omdat het anders alle bestandsnamen verraadt.
- **Controle zonder privésleutel:** de zip wordt in het geheugen gecontroleerd voordat hij versleuteld wordt, op CRC en SHA-256 per bestand. Daarna wordt het `.age`-bestand van de schijf teruggelezen en vergeleken met de SHA-256 die tijdens het schrijven over de versleutelde bytes is berekend. Zo is aangetoond dat wat op de schijf staat precies is wat er geschreven is.
- **Volledige controle met privésleutel:** `usb-backup-cli verify --identity sleutel.txt --all` ontsleutelt en pakt alles uit, bijvoorbeeld eens per maand handmatig. age verifieert elk blok van 64 KiB, dus elke beschadiging valt op.
- **Terugzetten:** `usb-backup-cli restore --identity sleutel.txt --week 2026-47 --to D:\Herstel`. Zonder de app kan het ook: `age -d -i sleutel.txt data.zip.age > data.zip`.

### Omgang met de privésleutel

De privésleutel bestaat alleen ontsleuteld in het geheugen van het CLI-proces, zo kort mogelijk, en wordt daarna gewist. Hij komt nooit op schijf, op het scherm, in een log of in een foutmelding.

- **Nooit op schijf:** geen tijdelijke bestanden met de sleutel of met ontsleutelde index of zip; ontsleutelen gebeurt als stroom naar de bestemming. Het geheugen met de sleutel wordt vastgezet met `VirtualLock`, zodat het niet naar het wisselbestand gaat.
- **Nooit zichtbaar:** de sleutel wordt nergens getoond, ook niet in `--json` of in logboeken. Uitzondering: het eenmalige break-glass-scherm bij het aanmaken. Wachtwoordvelden in de GUI tonen bolletjes en blokkeren kopiëren.
- **Wissen:** wachtwoord en sleutel staan in `[]byte`, nooit in een Go-`string` (die is onveranderlijk en kan niet gewist worden). Na gebruik worden ze overschreven met nullen, en het ontsleutelde vault-bestand ook. Het herstelscherm vergeet het wachtwoord na het herstellen, of na 5 minuten zonder activiteit.
- **Crashdumps:** de CLI meldt zich af bij Windows Foutrapportage (`WerAddExcludedApplication`) en zet `SetErrorMode`, zodat een crash geen geheugendump met de sleutel achterlaat.
- **Eerlijke grens:** Go kan geheugen kopiëren (garbage collector, groeiende slices), en de age-bibliotheek houdt intern een eigen kopie van de sleutel bij. Volledig wissen is in Go dus niet te garanderen. Daarom draait de sleutel in het kortlevende CLI-proces: als dat stopt, geeft Windows al het geheugen vrij. Wie het nog strakker wil, kan dat proces na elke herstelactie laten stoppen; dat is de standaard.

### Instellingen en exitcodes

| Sleutel of vlag | Standaard | Betekenis |
| --- | --- | --- |
| `encryption` | `age` als `usb-backup.recipients` bestaat, anders `none` | Versleuteling aan of uit |
| `recipientsFile` | `usb-backup.recipients` | Publieke sleutels, één per regel |
| `--identity <pad>` | geen | Privésleutel voor `verify` en `restore`; nooit opgeslagen |

Nieuwe exitcode: **13** betekent dat ontsleutelen mislukt is (verkeerde sleutel of beschadigd bestand). Staat versleuteling aan maar ontbreekt het sleutelbestand, dan stopt de back-up met code 10 en schrijft hij niets onversleuteld weg.

De belangrijkste waarschuwing: **zonder vault-wachtwoord én zonder break-glass-sleutel is de back-up waardeloos.** `keys init` toont de break-glass-sleutel daarom één keer en gaat pas verder na bevestiging dat hij bewaard is. `status` meldt het als er nog nooit met de break-glass-sleutel is getest.

## Commandline

Er zijn twee exe's. `usb-backup-cli.exe` doet al het werk: back-up, controleren, bladeren, herstellen, schijven, sleutels, planning. `usb-backup.exe` is alleen de GUI en roept onder water de CLI aan. Alles wat in de GUI kan, kan dus per definitie ook op de commandline.

- **Koppeling:** de GUI start `usb-backup-cli.exe <commando> --json-stream` als kindproces zonder venster. De CLI schrijft voortgang en resultaat als JSON-regels naar stdout (`{"event":"progress","tag":"Data","pct":42}`); de GUI tekent die. Annuleren gaat via een signaal op stdin.
- **Wachtwoord:** de GUI geeft het vault-wachtwoord via een pipe op stdin door, nooit als argument of omgevingsvariabele. De privésleutel zelf verlaat het CLI-proces nooit; de GUI ziet hem niet.
- **Taakplanner en scripts:** roepen de CLI direct aan. Die is een gewoon console-programma, dus cmd en PowerShell wachten erop en lezen de exitcode.
- **Wachtwoord op de commandline:** via `--password-stdin`, of de CLI vraagt erom zonder het te tonen. Geen argument, geen omgevingsvariabele.
- **Voorbeelden:** `usb-backup-cli restore --tag Boekhouding --week 2026-47 --file "Facturen\2026" --to D:\Hersteld` en `usb-backup-cli list --tag Data --week 2026-47 --path Documenten`

Een USB-trigger met de GUI start `usb-backup.exe --auto`, dat het venster toont en de CLI aanstuurt. Een trigger zonder gebruiker (geplande taak) start de CLI direct.

### Commando's

| Commando | Doel | Belangrijkste vlaggen |
| --- | --- | --- |
| `run` | Back-up maken van alle bronmappen, met dezelfde stappen als bij het inpluggen | `--target E:`, `--any-disk`, `--tag <tag>`, `--force`, `--wait`, `--dry-run`, `--quiet-skip`, `--json` |
| `verify` | Bestaande back-ups opnieuw controleren; met sleutel volledig uitpakken | `--target E:`, `--week 2026-47`, `--tag <tag>`, `--all`, `--password-stdin` of `--identity <pad>`, `--json` |
| `list` | Bladeren: weken, bronmappen en bestanden in een back-up tonen (leest de index) | `--week 2026-47`, `--tag <tag>`, `--path <map>`, `--search <tekst>`, `--password-stdin` of `--identity <pad>`, `--json` |
| `restore` | Back-up ontsleutelen en uitpakken naar een map; ook losse bestanden of mappen | `--week 2026-47`, `--tag <tag>`, `--file <pad>`, `--to <map>`, `--original` (terug naar de oorspronkelijke plek), `--password-stdin` of `--identity <pad>` |
| `status` | Aangesloten back-upschijven tonen: naam, gezondheid, vrije ruimte, weekmappen, laatste back-up | `--json` |
| `enroll` | Schijf registreren: ID-bestand schrijven, serienummers vastleggen | `<schijf>`, `--name BACKUP1`, `--weeks odd\|even` |
| `disks` | Geregistreerde en aangesloten schijven naast elkaar tonen | `--json` |
| `forget` | Registratie van een schijf verwijderen | `<naam>` |
| `keys init` | Operationeel en break-glass sleutelpaar maken (post-quantum), vault met wachtwoord aanmaken | `--vault <pad>`, `--breakglass-out <pad>` |
| `keys passwd` | Wachtwoord van de vault wijzigen | `--vault <pad>` |
| `keys rotate` | Nieuw operationeel sleutelpaar; oude back-ups blijven met break-glass te openen | `--vault <pad>` |
| `config` | Bronmappen en instellingen beheren | `add-source --tag --path`, `remove-source`, `set <sleutel> <waarde>`, `import`, `export` |
| `install` | Taak registreren | `--trigger usb` (standaard), `--trigger daily --at 02:00`, `--trigger weekly --day fri --at 18:00` |
| `uninstall` | Taak verwijderen | `--trigger usb\|daily\|weekly\|all` |
| `version`, `help` | Versie en uitleg tonen |  |

Voor alle commando's gelden ook: `--config <pad>`, `--log <pad>`, `--ui` of `--no-ui`, en `--quiet`.

- **`--target E:`:** gebruikt deze schijf. Het label moet nog steeds BACKUP1 of BACKUP2 zijn.
- **`--any-disk`:** laat de regel voor oneven en even weken los; elke back-upschijf is dan goed.
- **`--force`:** negeert de grens van 10 minuten.
- **`--wait <s>`:** hoe lang er op een schijfletter gewacht wordt. Standaard 0 bij de CLI-exe en 30 bij de trigger.
- **`--dry-run`:** toont schijf, doelmap, verwachte grootte en welke weekmappen de rotatie zou verwijderen, en schrijft niets.
- **`--quiet-skip`:** geen melding bij exitcode 2 tot en met 4 (geen schijf, verkeerde schijf, kort geleden al gedaan).
- **`--json`:** één JSON-object op stdout met dezelfde inhoud als `rapport.txt`, bijvoorbeeld voor monitoring.
- **Meldingen:** de CLI-exe toont standaard geen MessageBox (`--ui` zet ze aan); `usb-backup.exe` toont ze standaard wel.

### Exitcodes

| Code | Betekenis |
| --- | --- |
| 0 | Geslaagd |
| 1 | Geslaagd, met waarschuwingen (overgeslagen bestanden, gezondheid Warning) |
| 2 | Geen back-upschijf aangesloten |
| 3 | Verkeerde schijf voor deze week |
| 4 | Overgeslagen: binnen de ingestelde tijd al een back-up gemaakt |
| 5 | Er loopt al een back-up (andere instantie) |
| 6 | Onbekende schijf: ID-bestand, handtekening of serienummers kloppen niet |
| 7 | Instellingen of registratielijst gewijzigd zonder geldige handtekening van de operator |
| 10 | Schijfcontrole mislukt (ruimte, bestandssysteem, schrijfbaar, sleutelbestand ontbreekt) |
| 11 | Fout bij lezen of schrijven |
| 12 | Controle van de back-up mislukt |
| 13 | Ontsleutelen of opnieuw inpakken mislukt (verkeerde sleutel, verkeerd wachtwoord of beschadigd bestand) |
| 64 | Verkeerd gebruik (onbekend commando of onbekende vlag) |

Bij een geplande taak zijn de codes 2 tot en met 4 normaal (de schijf zit er gewoon niet in). Scripts kunnen ze daarom apart behandelen van echte fouten.

### Gepland draaien

Een "cronjob" is op Windows een tijdtrigger in Taakplanner. `install --trigger daily --at 02:00` registreert een tweede taak, "USB Backup (dagelijks)". Die start `usb-backup.exe run --quiet-skip`, naast de bestaande USB-trigger. Het schijfwisselen per week blijft zo gewoon werken. Zit de juiste schijf er al eerder die week in, dan wordt de weekmap bijgewerkt.

```
usb-backup-cli status
usb-backup-cli run --dry-run
usb-backup-cli run --target E: --force --json > rapport.json
usb-backup-cli verify --all
usb-backup-cli install --trigger daily --at 02:00
```

## Linux

`usb-backup-cli` draait ook op Linux, met dezelfde commando's, hetzelfde formaat op de USB-schijf en dezelfde sleutels. Een schijf die op Linux is gevuld, is op Windows te herstellen en andersom. De GUI blijft alleen voor Windows.

### Gebruik

```
sudo usb-backup-cli enroll /dev/sdb1 --name BACKUP1 --weeks odd
usb-backup-cli config add-source --tag Projecten --path /home/maurice/projecten
usb-backup-cli run --quiet-skip

# crontab: elk uur kijken of de juiste stick erin zit
0 * * * *  /usr/local/bin/usb-backup-cli run --quiet-skip
```

Zit de stick er niet in, dan stopt `run` met exitcode 2, zonder melding. De grens van 10 minuten en de weekmap zorgen dat een uurlijkse cron niet elk uur een nieuwe back-up maakt: binnen dezelfde week wordt de bestaande weekmap alleen bijgewerkt als de vorige back-up ouder is dan `minIntervalHours` (standaard 20).

### Direct bij inpluggen (optioneel)

In plaats van cron kan `usb-backup-cli install --trigger usb` op Linux een udev-regel en een systemd-service maken. udev herkent de geregistreerde schijf aan het filesystem-UUID en start `usb-backup@<uuid>.service`, die de back-up uitvoert. udev zelf mag geen lange taken draaien; daarom loopt het via systemd.

### Platformverschillen

| Onderdeel | Windows | Linux |
| --- | --- | --- |
| Schijven vinden | `GetVolumeInformationW`, schijfletters | `lsblk --json` en `/sys/block`, gekoppelde mappen uit `/proc/self/mountinfo` |
| Volume-ID | Volume-serienummer | Filesystem-UUID |
| Partitie-ID | Partitie-GUID of disk signature | `PARTUUID` |
| Hardware-serienummer | `IOCTL_STORAGE_QUERY_PROPERTY` | `ID_SERIAL` via udev (`/run/udev/data`) |
| Koppelen | Door Windows | Is de stick niet gekoppeld, dan via `udisksctl mount` (zonder root) of `mount` als root naar `/run/usb-backup/<uuid>`; na afloop weer ontkoppelen |
| Gezondheid | `MSFT_Disk` HealthStatus | `smartctl -H` als het geïnstalleerd is, anders onbekend |
| Eén tegelijk | Mutex `Global\UsbBackup` | `flock` op `/run/lock/usb-backup.lock` |
| Planning | Taakplanner | cron of systemd-timer; udev + systemd voor inpluggen |
| Configuratie | C:\\ProgramData\\USB Backup | `/etc/usb-backup/` als root, anders `~/.config/usb-backup/` |
| Sleutel in geheugen | `VirtualLock`, uitgesloten van Foutrapportage | `mlock`, `prctl(PR_SET_DUMPABLE, 0)` tegen coredumps |
| Bestandsrechten | Wijzigingsdatum | Ook eigenaar, groep en rechten in de zip; symlinks als link opgeslagen, niet gevolgd |

In de repo krijgt elk platformpakket een `_linux.go` naast `_windows.go`. De build krijgt er `GOOS=linux GOARCH=amd64` (en `arm64`, bijvoorbeeld voor een Raspberry Pi) bij, en de CI test ook op Linux met echte loop-devices als nep-USB.

## Installatiepakketten

Windows krijgt een `USB-Backup-Setup.exe` en Linux een `.deb`, een `.rpm` en een `.tar.gz`. Alle pakketten worden in de CI op Linux gebouwd, zonder Windows-machine.

### Windows

- **Gereedschap:** NSIS (`makensis` draait op Linux). Voor beheerde omgevingen (Intune, GPO) kan er later een MSI bij via `wixl` uit msitools.
- **Installeert:** beide exe's in `C:\Program Files\USB Backup\`, een snelkoppeling in het startmenu en optioneel op het bureaublad, `usb-backup-cli` in het PATH, en een vermelding in Apps en onderdelen.
- **Configuratie en logboek:** naar `C:\ProgramData\USB Backup\` in plaats van naast de exe, omdat Program Files niet schrijfbaar is voor gewone gebruikers. De vault blijft per gebruiker in `%APPDATA%\USB Backup\` of op een sleutel-USB.
- **Na installatie:** het instellingenscherm opent als wizard: bronmappen kiezen, schijven registreren, sleutels maken, USB-trigger aanzetten.
- **Stil installeren:** `USB-Backup-Setup.exe /S`, met daarna `usb-backup-cli config import instellingen.json` om een vooraf gemaakte configuratie uit te rollen (zonder sleutels).
- **Bijwerken:** een nieuwe setup over de oude heen; configuratie, registraties en vault blijven staan.
- **Verwijderen:** haalt de exe's en de taken in Taakplanner weg. Configuratie en vault blijven staan, tenzij je dat aanvinkt; de back-ups op de USB-schijven worden nooit aangeraakt.
- **Ondertekenen:** setup en exe's met een code-signing-certificaat via `osslsigncode` (ook op Linux), tegen SmartScreen-waarschuwingen. Zonder certificaat werkt het ook, maar met een waarschuwing bij de eerste start.

### Linux

- **Gereedschap:** nfpm (een Go-tool) maakt `.deb` en `.rpm` vanuit één beschrijving; daarnaast een `.tar.gz` met `install.sh` voor andere distributies.
- **Installeert:** `/usr/bin/usb-backup-cli`, `/etc/usb-backup/` voor de configuratie, de systemd-units `usb-backup@.service` en `usb-backup.timer`, de udev-regel, en een man-page.
- **Niets automatisch aan:** na installatie draait er nog niets. Inschakelen gebeurt bewust met `usb-backup-cli install --trigger usb` of `--trigger daily`, of met een eigen cronregel.
- **Afhankelijkheden:** geen verplichte; `udisks2` en `smartmontools` zijn aanbevolen.
- **Verwijderen:** `apt remove` of `dnf remove` haalt de timer en de udev-regel weg; met `purge` ook `/etc/usb-backup/`.

### Release

Bij een git-tag bouwt de CI alles in één keer (bijvoorbeeld met GoReleaser): de exe's voor Windows amd64 en arm64, de Linux-binaries voor amd64 en arm64, de setup, de pakketten, en een `SHA256SUMS` met handtekening. Alles komt als downloads bij de release op GitHub.

## Foutafhandeling en randgevallen

De vaste regel: bij twijfel wordt niets verwijderd, en een mislukte back-up laat geen half bestand achter.

| Situatie | Gedrag |
| --- | --- |
| Gebeurtenis 1006 komt meerdere keren, of ook bij het uittrekken | IgnoreNew zolang het programma loopt, plus `laatste-backup.txt` (10 minuten). Geen back-upschijf gevonden: stil stoppen |
| Geplande taak en inpluggen tegelijk | Benoemde mutex `Global\UsbBackup`: de tweede instantie stopt met exitcode 5 |
| Geplande taak terwijl er geen schijf in zit | Exitcode 2, geen melding (`--quiet-skip`) |
| Schijfletter nog niet toegewezen | Tot 30 s elke seconde opnieuw zoeken (bij de CLI-exe standaard 0 s) |
| Beide schijven tegelijk aangesloten | Alleen de schijf van deze week wordt gebruikt, tenzij `--any-disk` of `--target` is opgegeven |
| Zelfde schijf twee keer in dezelfde week | Nieuwe zip vervangt de oude, pas na een geslaagde controle |
| Bestand vergrendeld (bv. Outlook `.pst`) | Overslaan, opnemen in het rapport, gele melding, exitcode 1 |
| Leesfout midden in een bestand | Back-up mislukt, `.part` wordt verwijderd, exitcode 11 |
| Schijf eruit getrokken tijdens de back-up | Schrijffout, exitcode 11. Een achtergebleven `.part` wordt bij de volgende start opgeruimd |
| Controle mislukt | `.part` wordt verwijderd, geen rotatie, rode melding, exitcode 12 |
| `C:\Data` ontbreekt of is leeg | Ontbreekt: fout. Leeg: waarschuwing, geen zip |
| Week 53 | In jaren met 53 ISO-weken zijn week 53 en week 1 allebei oneven, dus is BACKUP1 twee keer achter elkaar aan de beurt. Eerstvolgende keer: eind 2026 naar begin 2027 |
| Paden langer dan 260 tekens | Go regelt het `\\?\`-voorvoegsel zelf; staat in het testplan |
| Systeemklok staat verkeerd | Het weeknummer volgt de klok; geen speciale afhandeling |

## Repo, build en tests

### Repo-structuur

De indeling volgt de gangbare Go-conventie: `cmd/` voor de programma's, `internal/` voor alle code die niet van buitenaf gebruikt mag worden, tests naast de code. Er is geen `pkg/`, omdat niets als bibliotheek bedoeld is.

```
usb-backup/
├─ cmd/
│  ├─ usb-backup-cli/        main.go: alleen argumenten doorgeven aan internal/cli
│  └─ usb-backup/            main.go: alleen de GUI starten (internal/gui)
├─ internal/
│  ├─ cli/                   commando's, vlaggen, exitcodes, --json, --json-stream
│  ├─ app/                   de stappen van run, verify, list, restore (zonder I/O-details)
│  ├─ archive/               zip, manifest, index
│  ├─ verify/                terugcontrole van zips
│  ├─ crypt/                 age-versleuteling, header rewrap
│  ├─ vault/                 vault met sleutelgeschiedenis
│  ├─ sign/                  Ed25519 voor config, registraties, ID-bestanden
│  ├─ secmem/                buffers wissen, lock, crashdumps uit
│  ├─ disk/                  registratie en herkenning
│  ├─ volume/                interface + volume_windows.go, volume_linux.go, fake.go
│  ├─ lock/                  lock_windows.go, lock_linux.go
│  ├─ task/                  Taakplanner, systemd, udev
│  ├─ rotate/                bewaarbeleid
│  ├─ week/                  periodes en schijfkeuze
│  ├─ config/                instellingen, validatie, paden per platform
│  ├─ logx/                  logboek
│  ├─ gui/                   windigo-vensters (alleen windows, build-tag)
│  └─ testutil/              nep-schijven, nep-klok, nep-CLI, golden files
├─ test/
│  └─ e2e/                   bouwt de echte binary en draait hem tegen nep-schijven
├─ testdata/                  vaste zips, configs, age-bestanden voor tests
├─ packaging/
│  ├─ windows/               NSIS-script, icoon, versieinfo
│  └─ linux/                 nfpm.yaml, systemd-units, udev-regel, man-page, install.sh
├─ scripts/                   hulpscripts voor handmatige tests (loop-device maken, enz.)
├─ docs/                      DESIGN.md, tasks.md, schermafbeeldingen
├─ .github/workflows/         ci.yml, release.yml
├─ .golangci.yml
├─ .goreleaser.yaml
├─ Makefile
├─ go.mod, go.sum
└─ README.md
```

- **`cmd/` blijft dun:** alleen `os.Exit(cli.Run(os.Args, os.Stdin, os.Stdout, os.Stderr))`. Daardoor is de hele CLI in tests aan te roepen zonder een proces te starten.
- **Afhankelijkheden van buiten zijn interfaces:** klok, schijven (`volume.Finder`), wachtwoordinvoer en taakregistratie. Productie gebruikt de echte, tests de nep-versie uit `testutil`.
- **Afhankelijkheden:** zo min mogelijk: `filippo.io/age`, windigo voor de GUI, en voor de rest de standaardbibliotheek.

### Build

```
VERSION=$(git describe --tags)

# CLI: console-programma, doet al het werk
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=$VERSION" \
  -o dist/usb-backup-cli.exe ./cmd/usb-backup-cli

# GUI: geen consolevenster, start de CLI als kindproces
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-H windowsgui -s -w -X main.version=$VERSION" \
  -o dist/usb-backup.exe ./cmd/usb-backup
```

Beide exe's staan altijd samen in dezelfde map; de GUI controleert bij het starten of de CLI dezelfde versie heeft.

Voor ARM-laptops wordt `GOARCH=arm64` gebruikt. Een icoon en versie-informatie kunnen later met een `.syso`-bestand (bijvoorbeeld via goversioninfo). Een niet-ondertekende exe geeft bij de eerste start een SmartScreen-waarschuwing.

### Testplan

Alle tests draaien met gewoon `go test` op de eigen machine, zonder Docker of virtuele machines. Schijven, klok en wachtwoorden zijn in tests nep, zodat bijna alles op Linux én Windows te testen is.

| Laag | Waar | Draaien | Wat |
| --- | --- | --- | --- |
| Unit | `*_test.go` naast de code | `make test` (`go test ./...`) | Pure logica: week, rotatie, zip, hashes, config, handtekeningen. Tabeltests, `t.TempDir()` |
| Integratie | `*_integration_test.go`, build-tag `integration` | `make test-integration` | Een hele `run`, `restore` of `keys rewrap` via `cli.Run` tegen nep-schijven (mappen in `t.TempDir()`) en een nep-klok |
| End-to-end | `test/e2e/` | `make test-e2e` | Bouwt de echte binary, draait hem als proces, controleert uitvoer, exitcodes en bestanden op de nep-schijf |
| Lang | build-tag `long` | `make test-long` | Zip64 boven 4 GB, grote aantallen bestanden |
| Handmatig | `scripts/` + checklist in `docs/` | zelf | Echte USB-schijven, Taakplanner, udev, GUI, installers |

- **Nep-schijven:** `testutil.FakeDisks(t, "BACKUP1", "BACKUP2")` maakt mappen met eigen volume-serienummer, partitie-ID, vrije ruimte en gezondheid. De echte binary gebruikt ze via `USB_BACKUP_FAKE_DISKS`, alleen in een testbuild (build-tag `fakes`), zodat het in productie niet kan.
- **Nep-klok:** `USB_BACKUP_FAKE_NOW` in testbuilds, om week 53, jaargrenzen en `minIntervalHours` te testen.
- **Golden files:** verwachte uitvoer (rapport, JSON, taak-XML, systemd-units) in `testdata/`; bijwerken met `go test ./... -update`.
- **Snelle sleutels:** tests gebruiken vaste testsleutels uit `testdata/` en een lage scrypt-instelling, zodat de testset binnen een minuut klaar is.
- **Dekking:** `make cover` toont de dekking; doel 80% voor `internal/` exclusief `gui` en de platform-API-code.
- **CI:** dezelfde Make-doelen op ubuntu-latest en windows-latest. Linux-specifieke tests met een echt loop-device draaien alleen in CI (met sudo) en worden lokaal overgeslagen als er geen rechten zijn.

* **Unit (alle platforms):**
  - Week naar schijf, inclusief 2026-W53 en 2027-W01, en de andere schema's (dag, maand, roundRobin)
  - Zip en controle heen en terug: meerdere bronmappen, geneste en lege mappen, Unicode-namen, bestanden van 0 bytes; zip64 boven 4 GB in een nachtelijke run
  - Controle vangt één gewijzigde byte en een ontbrekende entry
  - age: versleutelen en ontsleutelen met operationele en break-glass-sleutel; mix van sleuteltypes geweigerd
  - Rewrap: header opnieuw inpakken, controleren tegen filippo.io/age, oude sleutel werkt daarna niet meer
  - Vault: verkeerd wachtwoord, sleutelgeschiedenis, buffers na gebruik nul
  - Ondertekening: gewijzigde config, registratielijst of ID-bestand wordt geweigerd (exitcode 7 of 6)
  - Rotatie: 27 mappen worden 25, over een jaargrens heen, met afwijkend patroon blijven andere mappen staan
  - Index en herstel: los bestand terugzetten, hash klopt, terugvallen zonder index
* **Integratie:** volledige run met twee nep-schijven (mappen op Windows, loop-devices op Linux), verkeerde schijf, gekopieerd ID-bestand, te weinig ruimte, onderbroken schrijven, sleutelrotatie over twee schijven
* **GUI:** toestanden van het statusscherm met een nep-CLI die JSON-regels afspeelt; operatorscherm sluit na de time-out
* **CI:** `go test ./...` op ubuntu-latest en windows-latest; bij een tag bouwt GoReleaser exe's, setup en pakketten
* **Handmatig:** echte schijven in- en uitpluggen, dubbele triggers, vergrendeld `.pst`, schijf eruit tijdens back-up, setup installeren, bijwerken en verwijderen, Linux met cron en met udev, slaapstand tijdens herstel

### Open vragen

- [ ] Moet BitLocker To Go op beide schijven er ook bij, als tweede laag naast age?
- [ ] Vergrendelde bestanden toch meenemen via een VSS-snapshot (Windows) of LVM/btrfs-snapshot (Linux)? Dat vraagt adminrechten.
- [ ] Het teruglezen na het schrijven buiten de cache om doen (`FILE_FLAG_NO_BUFFERING`, `O_DIRECT`)?
- [ ] Mag de vault ook op een aparte sleutel-USB staan in plaats van op de pc?
- [ ] Code-signing-certificaat aanschaffen voor de Windows-setup, of de SmartScreen-waarschuwing accepteren?
- [ ] Overstappen op ML-DSA voor de ondertekening zodra dat in de Go-standaardbibliotheek zit?
