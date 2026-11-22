package secmem

import (
	"runtime"
	"testing"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

func TestWipe(t *testing.T) {
	src := []byte("correct horse battery staple")
	b := From(src)
	for _, c := range src {
		if c != 0 {
			t.Fatal("source not wiped")
		}
	}
	if string(b.Bytes()) != "correct horse battery staple" {
		t.Fatal("copy wrong")
	}
	full := b.Bytes()[:cap(b.Bytes())]
	s := UnsafeString(b.Bytes())
	b.Wipe()
	for _, c := range full {
		if c != 0 {
			t.Fatal("buffer not all zeros after Wipe")
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] != 0 {
			t.Fatal("unsafe string still holds the secret")
		}
	}
	if b.Len() != 0 || b.Locked() {
		t.Fatal("state after wipe")
	}
	b.Wipe()
	var nb *Buffer
	nb.Wipe()
}

func TestTruncateAndEqual(t *testing.T) {
	b := From([]byte("secret\n"))
	full := b.Bytes()
	b.Truncate(6)
	if string(b.Bytes()) != "secret" || full[6] != 0 {
		t.Fatal("truncate")
	}
	if !b.Equal(From([]byte("secret"))) || b.Equal(From([]byte("secreT"))) {
		t.Fatal("equal")
	}
}

func TestProtectProcess(t *testing.T) {
	if err := ProtectProcess(); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" {
		d, err := dumpable()
		if err != nil || d {
			t.Fatalf("still dumpable: %v %v", d, err)
		}
	}
}
