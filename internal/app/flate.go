package app

import (
	"compress/flate"
	"io"
)

func flateReader(r io.Reader) io.ReadCloser { return flate.NewReader(r) }
