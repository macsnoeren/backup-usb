package archive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// WriteFileTo is a test helper: it writes dest+".part" through a root on
// the folder of dest.
func WriteFileTo(t testing.TB, src, dest string, opts Options) (*Result, error) {
	return WriteFileToCtx(t, context.Background(), src, dest, opts)
}

// WriteFileToCtx is WriteFileTo with a context.
func WriteFileToCtx(t testing.TB, ctx context.Context, src, dest string, opts Options) (*Result, error) {
	t.Helper()
	root, err := os.OpenRoot(filepath.Dir(dest))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	return WriteFile(ctx, src, root, filepath.Base(dest), opts)
}
