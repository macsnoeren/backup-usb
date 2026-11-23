package crypt

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"filippo.io/age"
)

func encryptSample(t *testing.T, rs []age.Recipient) []byte {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, rs...)
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(strings.Repeat("payload ", 20000)))
	w.Close()
	return buf.Bytes()
}

func TestHeaderRoundTripIsByteEqual(t *testing.T) {
	data := encryptSample(t, recipients(t))
	h, payload, err := ReadHeader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Stanzas) != 2 {
		t.Fatalf("%d stanzas", len(h.Stanzas))
	}
	rest, _ := io.ReadAll(payload)
	again := append(h.Marshal(), rest...)
	if !bytes.Equal(again, data) {
		t.Fatal("re-written header differs")
	}
	// The library agrees on where the header ends.
	lib, _ := age.ExtractHeader(bytes.NewReader(data))
	if !bytes.Equal(lib, h.Marshal()) {
		t.Fatal("header differs from age.ExtractHeader")
	}
}

func TestReplaceStanzasAndMAC(t *testing.T) {
	data := encryptSample(t, recipients(t))
	op := identity(t, "operational")
	fileKey, err := age.DecryptHeader(mustExtract(t, data), op)
	if err != nil {
		t.Fatal(err)
	}
	h, payload, _ := ReadHeader(bytes.NewReader(data))
	if !h.CheckMAC(fileKey) {
		t.Fatal("MAC of the original header does not check")
	}
	// Re-wrap for operational2 + break-glass.
	r2, _ := age.ParseHybridRecipient(testutilRecipient(t, "operational2"))
	bg, _ := age.ParseHybridRecipient(testutilRecipient(t, "breakglass"))
	h.Stanzas = nil
	for _, r := range []age.Recipient{r2, bg} {
		s, err := r.Wrap(fileKey)
		if err != nil {
			t.Fatal(err)
		}
		h.Stanzas = append(h.Stanzas, s...)
	}
	if err := h.SetMAC(fileKey); err != nil {
		t.Fatal(err)
	}
	rest, _ := io.ReadAll(payload)
	out := append(h.Marshal(), rest...)
	for _, key := range []string{"operational2", "breakglass"} {
		r, err := age.Decrypt(bytes.NewReader(out), identity(t, key))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if b, _ := io.ReadAll(r); !bytes.Equal(b, []byte(strings.Repeat("payload ", 20000))) {
			t.Fatal("payload changed")
		}
	}
	if _, err := age.Decrypt(bytes.NewReader(out), op); err == nil {
		t.Fatal("old key still works")
	}
	// A wrong MAC is refused by the library.
	h.MAC[0] ^= 1
	if _, err := age.Decrypt(bytes.NewReader(append(h.Marshal(), rest...)), identity(t, "operational2")); err == nil {
		t.Fatal("bad MAC accepted")
	}
}

func TestParseHeaderErrors(t *testing.T) {
	good := string(mustExtract(t, encryptSample(t, recipients(t))))
	bad := []string{
		"", "age-encryption.org/v2\n", "age-encryption.org/v1\n--- AAAA\n",
		strings.Replace(good, "-> ", "->  ", 1),
		strings.Replace(good, "--- ", "---", 1),
		"age-encryption.org/v1\n-> X\n" + strings.Repeat("A", 65) + "\n--- x\n",
		good[:len(good)-10],
	}
	for _, b := range bad {
		if _, err := ParseHeader(bufio.NewReader(strings.NewReader(b))); err == nil {
			t.Errorf("accepted %q", b[:min(len(b), 40)])
		}
	}
}

func TestHugeHeaderLine(t *testing.T) {
	data := append([]byte("age-encryption.org/v1\n-> X25519 "), bytes.Repeat([]byte("A"), 8<<20)...)
	if _, err := ParseHeader(bufio.NewReaderSize(bytes.NewReader(data), 64<<10)); err == nil {
		t.Fatal("huge line accepted")
	}
}

func mustExtract(t *testing.T, data []byte) []byte {
	h, err := age.ExtractHeader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func FuzzParseHeader(f *testing.F) {
	var buf bytes.Buffer
	x, _ := age.GenerateX25519Identity()
	w, _ := age.Encrypt(&buf, x.Recipient())
	w.Close()
	f.Add(buf.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		h, err := ParseHeader(bufio.NewReader(bytes.NewReader(data)))
		if err != nil {
			return
		}
		// What parses must re-encode to a prefix of the input.
		if m := h.Marshal(); !bytes.HasPrefix(data, m) {
			t.Fatalf("re-encoded header is not the input")
		}
	})
}
