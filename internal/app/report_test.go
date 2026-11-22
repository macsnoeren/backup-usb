package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func sampleReport() *RunReport {
	start := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	return &RunReport{
		Status: StatusWarning, Disk: "BACKUP1", Device: "E:", Mount: `E:\`, Period: "2026-W47", Folder: "2026/47",
		Started: start, Finished: start.Add(95 * time.Second), Health: "Healthy", FreeBytes: 40 << 30, FolderCount: 25,
		Rotated: []string{"2026/21"},
		Sources: []SourceReport{
			{Tag: "Data", Status: StatusWarning, Archive: "Data.zip", Files: 1200, Dirs: 80, Bytes: 3 << 30, ZipBytes: 2 << 30,
				SHA256: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", Verified: true,
				SkippedCount: 1, Skipped: []archive.Skipped{{Path: "Outlook/archive.pst", Reason: "in use or not readable"}}},
			{Tag: "Books", Status: StatusOK, Archive: "Books.zip", Files: 12, Dirs: 2, Bytes: 4096, ZipBytes: 2048,
				SHA256: "486ea46224d1bb4fb680f34f7c9ad96a8f24ec88be73ea8e5a6c65260e9cb8a7", Verified: true},
		},
		Messages:    []string{"Verification OK. You can remove the disk."},
		MessageKeys: []string{"msg.canRemove"},
	}
}

func TestReportGolden(t *testing.T) {
	r := sampleReport()
	testutil.Golden(t, "report-warning.txt", []byte(RenderReport(r, msg.EN)))
	testutil.Golden(t, "report-warning-nl.txt", []byte(RenderReport(r, msg.NL)))
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Golden(t, "report-warning.json", append(data, '\n'))

	// Encrypted: no file names in the report.
	r.Encrypted = true
	r.Keys = []string{"4f1a9c0d2b3e5f67", "a1b2c3d4e5f60718"}
	r.Redact()
	text := RenderReport(r, msg.EN)
	testutil.Golden(t, "report-encrypted.txt", []byte(text))
	data, _ = json.MarshalIndent(r, "", "  ")
	testutil.Golden(t, "report-encrypted.json", append(data, '\n'))
	for _, s := range []string{string(data), text} {
		if contains(s, "archive.pst") {
			t.Fatal("file name in an encrypted report")
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
