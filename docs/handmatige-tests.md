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
