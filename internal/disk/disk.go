package disk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/volume"
)

// IDDir and IDFileName locate the ID file on a disk: <root>/Backup/disk-id.json.
// It is independent of the targetFolder setting, so changing that setting
// never makes a disk unknown.
const (
	IDDir      = "Backup"
	IDFileName = "disk-id.json"
	// MaxFileSize bounds the ID file and the registration list.
	MaxFileSize = 256 << 10
)

// Registration is one enrolled disk in usb-backup.disks.json.
type Registration struct {
	Name           string `json:"name"`
	Weeks          string `json:"weeks,omitempty"`
	ID             string `json:"id"`
	VolumeSerial   string `json:"volumeSerial"`
	PartitionID    string `json:"partitionId"`
	HardwareSerial string `json:"hardwareSerial,omitempty"`
	Enrolled       string `json:"enrolled"`
}

// Registry is the registration list.
type Registry struct {
	Version int            `json:"version"`
	Disks   []Registration `json:"disks"`
}

// IDFile is Backup/disk-id.json on a disk. It contains no secrets.
type IDFile struct {
	Version        int    `json:"version"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	VolumeSerial   string `json:"volumeSerial"`
	PartitionID    string `json:"partitionId"`
	HardwareSerial string `json:"hardwareSerial,omitempty"`
	Enrolled       string `json:"enrolled"`
	Signature      string `json:"signature,omitempty"`
}

var (
	nameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
	uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// ValidName reports whether name is a valid disk name.
func ValidName(name string) bool { return nameRe.MatchString(name) }

// ValidID reports whether id is a lower-case UUID.
func ValidID(id string) bool { return uuidRe.MatchString(id) }

// ParseRegistry decodes a registration list strictly.
func ParseRegistry(data []byte) (*Registry, error) {
	r := &Registry{}
	if len(bytes.TrimSpace(data)) == 0 {
		return r, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(r); err != nil {
		return nil, fmt.Errorf("registration list: %w", err)
	}
	seenName, seenID := map[string]bool{}, map[string]bool{}
	for _, d := range r.Disks {
		if !ValidName(d.Name) || !ValidID(d.ID) {
			return nil, fmt.Errorf("registration list: invalid entry %q", d.Name)
		}
		if d.Weeks != "" && d.Weeks != "odd" && d.Weeks != "even" {
			return nil, fmt.Errorf("registration list: invalid weeks %q for %s", d.Weeks, d.Name)
		}
		if seenName[strings.ToLower(d.Name)] || seenID[d.ID] {
			return nil, fmt.Errorf("registration list: %s registered twice", d.Name)
		}
		seenName[strings.ToLower(d.Name)], seenID[d.ID] = true, true
	}
	return r, nil
}

// Marshal encodes the registry as indented JSON.
func (r *Registry) Marshal() ([]byte, error) {
	if r.Version == 0 {
		r.Version = 1
	}
	if r.Disks == nil {
		r.Disks = []Registration{}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode registration list: %w", err)
	}
	return append(data, '\n'), nil
}

// Find returns the registration with the given name (case-insensitive).
func (r *Registry) Find(name string) (*Registration, int) {
	for i := range r.Disks {
		if strings.EqualFold(r.Disks[i].Name, name) {
			return &r.Disks[i], i
		}
	}
	return nil, -1
}

// ByID returns the registration with the given ID.
func (r *Registry) ByID(id string) (*Registration, int) {
	for i := range r.Disks {
		if r.Disks[i].ID == id {
			return &r.Disks[i], i
		}
	}
	return nil, -1
}

// IDPath returns the ID file path on a mounted volume.
func IDPath(mount string) string { return filepath.Join(mount, IDDir, IDFileName) }

// ErrNoIDFile means the disk has no ID file: an ordinary USB stick, which
// is ignored silently.
var ErrNoIDFile = errors.New("no ID file")

// ReadIDFile reads and parses the ID file of a mounted volume. The file is
// untrusted input.
func ReadIDFile(mount string) (*IDFile, []byte, error) {
	f, err := os.Open(IDPath(mount))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrNoIDFile
	}
	if err != nil {
		return nil, nil, fmt.Errorf("open ID file: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read ID file: %w", err)
	}
	if len(data) > MaxFileSize {
		return nil, nil, errors.New("ID file too large")
	}
	id, err := ParseIDFile(data)
	return id, data, err
}

// ParseIDFile decodes an ID file strictly.
func ParseIDFile(data []byte) (*IDFile, error) {
	var id IDFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&id); err != nil {
		return nil, fmt.Errorf("ID file: %w", err)
	}
	if !ValidID(id.ID) || !ValidName(id.Name) {
		return nil, errors.New("ID file: invalid id or name")
	}
	return &id, nil
}

// Mismatch explains why a disk is not recognised.
type Mismatch struct{ Reason string }

func (m *Mismatch) Error() string { return "unknown disk: " + m.Reason }

// Match checks a connected volume and its ID file against the registry. It
// returns the registration, or a *Mismatch. requireHW refuses disks
// registered without a hardware serial number.
func Match(v volume.Volume, id *IDFile, reg *Registry, requireHW bool) (*Registration, error) {
	r, _ := reg.ByID(id.ID)
	if r == nil {
		return nil, &Mismatch{"ID not registered"}
	}
	if !strings.EqualFold(r.Name, id.Name) {
		return nil, &Mismatch{"name differs from registration"}
	}
	if !sameSerial(r.VolumeSerial, v.VolumeSerial) || !sameSerial(id.VolumeSerial, v.VolumeSerial) {
		return nil, &Mismatch{"volume serial number differs"}
	}
	if !sameSerial(r.PartitionID, v.PartitionID) || !sameSerial(id.PartitionID, v.PartitionID) {
		return nil, &Mismatch{"partition ID differs"}
	}
	if r.HardwareSerial != "" {
		if !sameSerial(r.HardwareSerial, v.HardwareSerial) || !sameSerial(id.HardwareSerial, v.HardwareSerial) {
			return nil, &Mismatch{"hardware serial number differs"}
		}
	} else if requireHW {
		return nil, &Mismatch{"no hardware serial number registered (requireHardwareSerial)"}
	}
	return r, nil
}

func sameSerial(a, b string) bool {
	return a != "" && strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
