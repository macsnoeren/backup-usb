package lock

import "errors"

func mkfifo(string) error { return errors.New("no FIFOs") }
