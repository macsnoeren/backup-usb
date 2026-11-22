package app

import (
	"crypto/ed25519"
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/disk"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/sign"
)

// SigningKey returns the operator's public signing key, or nil before
// "keys init" (then nothing is checked yet).
func SigningKey(dir string) (ed25519.PublicKey, error) {
	data, err := config.ReadLimited(filepath.Join(dir, config.SigningPubFile), 4096)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, Errorf(KindTampered, err, "err.tampered")
	}
	pub, err := sign.ParsePublicKey(data)
	if err != nil {
		return nil, Errorf(KindTampered, err, "err.tampered")
	}
	return pub, nil
}

// readSigned reads a file and, when a signing key exists, checks its .sig
// file. The data is read once and returned, so what is checked is what is
// used. A missing file gives fs.ErrNotExist.
func readSigned(path string, maxSize int64, pub ed25519.PublicKey, purpose string) ([]byte, error) {
	data, err := config.ReadLimited(path, maxSize)
	if err != nil {
		return nil, err
	}
	if pub == nil {
		return data, nil
	}
	sig, err := config.ReadLimited(path+config.SignatureSuffix, 4096)
	if err != nil {
		// Do not wrap err: a missing signature must never look like a
		// missing file (which would fall back to the defaults).
		return nil, Errorf(KindTampered, errors.New("signature file missing or unreadable"), "err.tampered")
	}
	if err := sign.VerifySigFile(pub, purpose, data, sig); err != nil {
		return nil, Errorf(KindTampered, err, "err.tampered")
	}
	return data, nil
}

// LoadConfig reads the settings, checking the operator's signature. A
// missing file gives the defaults.
func LoadConfig(dir string) (*config.Config, error) {
	pub, err := SigningKey(dir)
	if err != nil {
		return nil, err
	}
	data, err := readSigned(config.Path(dir), config.MaxFileSize, pub, sign.PurposeConfig)
	if errors.Is(err, fs.ErrNotExist) {
		return config.Defaults(), nil
	}
	if err != nil {
		return nil, err
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return nil, Errorf(KindUsage, err, "err.config", err.Error())
	}
	return cfg, nil
}

// writeSigned writes data and its .sig (when a key is given).
func writeSigned(path string, data []byte, key *secmem.Buffer, purpose string) error {
	if key != nil {
		sig, err := sign.SigFile(key.Bytes(), purpose, data)
		if err != nil {
			return Errorf(KindIO, err, "err.write")
		}
		// Signature first: a crash in between leaves the old data with a
		// signature that does not fit (exit 7), never new unsigned data
		// that is accepted.
		if err := fsx.WriteFileAtomic(path+config.SignatureSuffix, sig, 0o644); err != nil {
			return Errorf(KindIO, err, "err.write")
		}
	}
	if err := fsx.WriteFileAtomic(path, data, 0o644); err != nil {
		return Errorf(KindIO, err, "err.write")
	}
	return nil
}

// SaveConfig validates, writes and signs the settings.
func SaveConfig(dir string, cfg *config.Config, key *secmem.Buffer) error {
	if err := cfg.Validate(config.ValidateOptions{}); err != nil {
		return Errorf(KindUsage, err, "err.config", err.Error())
	}
	data, err := cfg.Marshal()
	if err != nil {
		return Errorf(KindIO, err, "err.write")
	}
	return writeSigned(config.Path(dir), data, key, sign.PurposeConfig)
}

// SaveRegistry writes and signs the registration list.
func SaveRegistry(dir string, cfg *config.Config, reg *disk.Registry, key *secmem.Buffer) error {
	data, err := reg.Marshal()
	if err != nil {
		return Errorf(KindIO, err, "err.write")
	}
	return writeSigned(cfg.DisksPath(dir), data, key, sign.PurposeRegistry)
}

// signExisting signs settings and registration list that exist already
// (used by keys init).
func signExisting(dir string, cfg *config.Config, key *secmem.Buffer) error {
	for _, f := range []struct{ path, purpose string }{{config.Path(dir), sign.PurposeConfig}, {cfg.DisksPath(dir), sign.PurposeRegistry}} {
		data, err := config.ReadLimited(f.path, config.MaxFileSize)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Errorf(KindIO, err, "err.read")
		}
		if err := writeSigned(f.path, data, key, f.purpose); err != nil {
			return err
		}
	}
	return nil
}
