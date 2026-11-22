package password

import (
	"errors"
	"io"
	"strings"
	"testing"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

func TestReadLineFromPipe(t *testing.T) {
	r, w := io.Pipe()
	go func() {
		w.Write([]byte("hunter2-long-pass\r\n{\"cancel\":true}\n"))
		w.Close()
	}()
	b, err := ReadLine(r)
	if err != nil || string(b.Bytes()) != "hunter2-long-pass" {
		t.Fatalf("%q %v", b.Bytes(), err)
	}
	// Nothing after the line was consumed.
	rest, _ := io.ReadAll(r)
	if string(rest) != "{\"cancel\":true}\n" {
		t.Fatalf("rest %q", rest)
	}
	b.Wipe()
}

func TestReadLineErrors(t *testing.T) {
	if _, err := ReadLine(strings.NewReader("\n")); !errors.Is(err, ErrEmpty) {
		t.Fatal(err)
	}
	if _, err := ReadLine(strings.NewReader("")); !errors.Is(err, ErrEmpty) {
		t.Fatal(err)
	}
	if _, err := ReadLine(strings.NewReader(strings.Repeat("x", MaxLen+1))); !errors.Is(err, ErrTooLong) {
		t.Fatal(err)
	}
	if b, err := ReadLine(strings.NewReader("no newline")); err != nil || string(b.Bytes()) != "no newline" {
		t.Fatal(err)
	}
}

func TestNewPassword(t *testing.T) {
	ok := Reader{strings.NewReader("twelve chars!\ntwelve chars!\n")}
	if b, err := NewPassword(ok, "", ""); err != nil || string(b.Bytes()) != "twelve chars!" {
		t.Fatal(err)
	}
	if _, err := NewPassword(Reader{strings.NewReader("short\nshort\n")}, "", ""); !errors.Is(err, ErrTooShort) {
		t.Fatal(err)
	}
	if _, err := NewPassword(Reader{strings.NewReader("twelve chars!\ntwelve chars?\n")}, "", ""); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
	// Twelve characters, not bytes.
	if _, err := NewPassword(Reader{strings.NewReader("ééééééééééé\nééééééééééé\n")}, "", ""); !errors.Is(err, ErrTooShort) {
		t.Fatal("11 runes accepted")
	}
}
