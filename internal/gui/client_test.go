// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package gui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

// The test binary doubles as a fake usb-backup-cli: USB_BACKUP_FAKE_CLI
// selects what it plays.
func TestMain(m *testing.M) {
	switch os.Getenv("USB_BACKUP_FAKE_CLI") {
	case "":
		os.Exit(m.Run())
	case "version":
		fmt.Println(`{"version": "` + os.Getenv("FAKE_VERSION") + `"}`)
		os.Exit(0)
	case "stream":
		in := bufio.NewReader(os.Stdin)
		if hasArg("--password-stdin") {
			pw, _ := in.ReadString('\n')
			if strings.TrimSpace(pw) != "secret-password" {
				fmt.Println(`{"event":"error","code":13,"key":"err.vaultPassword","message":"wrong vault password"}`)
				os.Exit(13)
			}
		}
		for _, l := range strings.Split(os.Getenv("FAKE_LINES"), "|") {
			if l == "WAITCANCEL" {
				line, _ := in.ReadString('\n')
				if strings.Contains(line, "cancel") {
					fmt.Println(`{"event":"error","code":11,"key":"err.cancelled","message":"cancelled"}`)
					os.Exit(11)
				}
				continue
			}
			fmt.Println(l)
			time.Sleep(5 * time.Millisecond)
		}
		os.Exit(0)
	case "crash":
		fmt.Println(`{"event":"state","state":"backup"}`)
		os.Exit(2)
	}
}

func hasArg(a string) bool {
	for _, x := range os.Args {
		if x == a {
			return true
		}
	}
	return false
}

func fakeCLI(t *testing.T, mode string, env ...string) CLI {
	t.Setenv("USB_BACKUP_FAKE_CLI", mode)
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		t.Setenv(k, v)
	}
	return CLI{Path: os.Args[0]}
}

func TestVersionCheck(t *testing.T) {
	c := fakeCLI(t, "version", "FAKE_VERSION=1.2.3")
	if err := c.CheckVersion(context.Background(), "1.2.3"); err != nil {
		t.Fatal(err)
	}
	err := c.CheckVersion(context.Background(), "1.2.4")
	if !errors.Is(err, ErrVersionMismatch) || !strings.Contains(err.Error(), "the window is 1.2.4, usb-backup-cli is 1.2.3") {
		t.Fatalf("err = %v", err)
	}
	if err := (CLI{Path: "/nonexistent/usb-backup-cli"}).CheckVersion(context.Background(), "1"); err == nil {
		t.Fatal("missing CLI accepted")
	}
}

func TestStreamJob(t *testing.T) {
	lines := `{"event":"state","state":"searching"}|{"event":"state","state":"backup","tag":"Data","index":1,"count":2}|{"event":"progress","tag":"Data","pct":50}|not json|{"event":"result","code":0,"report":{"status":"ok"}}`
	c := fakeCLI(t, "stream", "FAKE_LINES="+lines)
	j, err := c.Start(context.Background(), []byte("secret-password"), "run")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for e := range j.Events {
		got = append(got, e.Event+":"+e.State)
	}
	if strings.Join(got, ",") != "state:searching,state:backup,progress:,result:" {
		t.Fatal(got)
	}
	if r := j.Wait(); r.Event != "result" || r.Code != 0 || !strings.Contains(string(r.Report), "ok") {
		t.Fatalf("%+v", r)
	}
}

func TestStreamCancelAndCrash(t *testing.T) {
	c := fakeCLI(t, "stream", "FAKE_LINES="+`{"event":"state","state":"backup"}|WAITCANCEL`)
	j, err := c.Start(context.Background(), nil, "run")
	if err != nil {
		t.Fatal(err)
	}
	<-j.Events
	j.Cancel()
	if r := j.Wait(); r.Code != 11 || r.Key != "err.cancelled" {
		t.Fatalf("%+v", r)
	}
	c = fakeCLI(t, "crash")
	j, _ = c.Start(context.Background(), nil, "run")
	for range j.Events {
	}
	if r := j.Wait(); r.Event != "error" || r.Code != 2 {
		t.Fatalf("%+v", r)
	}
	c = fakeCLI(t, "stream", "FAKE_LINES=")
	j, _ = c.Start(context.Background(), []byte("wrong"), "list")
	for range j.Events {
	}
	if r := j.Wait(); r.Code != 13 {
		t.Fatalf("%+v", r)
	}
}
