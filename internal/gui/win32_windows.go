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
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/rodrigocfd/windigo/win"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex      = kernel32.NewProc("CreateMutexW")
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procShellExecute     = shell32.NewProc("ShellExecuteW")
	procShellExecuteEx   = shell32.NewProc("ShellExecuteExW")
	procSHBrowseFolder   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDL = shell32.NewProc("SHGetPathFromIDListW")
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	procGetSaveFileName  = comdlg32.NewProc("GetSaveFileNameW")
	procGetOpenFileName  = comdlg32.NewProc("GetOpenFileNameW")
	ole32                = syscall.NewLazyDLL("ole32.dll")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
)

const errorAlreadyExists = 183

// singleInstance takes a GUI mutex; false means another window has it.
func singleInstance(mutex string) bool {
	name, _ := syscall.UTF16PtrFromString(`Local\` + mutex)
	h, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name))) //nolint:gosec // Win32 call
	if h == 0 {
		return true
	}
	return !errors.Is(err, syscall.Errno(errorAlreadyExists))
}

// shellExecute opens a file with its program ("open") or prints it
// ("print"). The file path is passed as one argument, never through a
// shell command line.
func shellExecute(verb, file string) error {
	v, _ := syscall.UTF16PtrFromString(verb)
	f, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	r, _, _ := procShellExecute.Call(0, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(f)), 0, 0, 1) //nolint:gosec // Win32 call
	if r <= 32 {
		return errors.New("cannot open " + filepath.Base(file))
	}
	return nil
}

type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	owner      uintptr
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instApp    uintptr
	idList     uintptr
	class      *uint16
	keyClass   uintptr
	hotKey     uint32
	icon       uintptr
	process    uintptr
}

// errDeclined means that the user did not give permission.
var errDeclined = errors.New("permission not given")

const (
	seeMaskNoCloseProcess = 0x40
	swShowNormal          = 1
	errorCancelled        = 1223
	asfwAny               = 0xFFFFFFFF
)

// startElevated starts exe with one fixed argument as administrator, after
// Windows asked for permission. The returned function waits until it ends.
func startElevated(owner uintptr, exe, arg string) (func(), error) {
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return nil, err
	}
	param, err := syscall.UTF16PtrFromString(arg)
	if err != nil {
		return nil, err
	}
	_ = win.AllowSetForegroundWindow(asfwAny) // the new window may come to the front
	info := shellExecuteInfo{mask: seeMaskNoCloseProcess, owner: owner, verb: verb, file: file, parameters: param, show: swShowNormal}
	info.size = uint32(unsafe.Sizeof(info))
	r, _, err := procShellExecuteEx.Call(uintptr(unsafe.Pointer(&info))) //nolint:gosec // Win32 call
	if r == 0 {
		if errors.Is(err, syscall.Errno(errorCancelled)) {
			return nil, errDeclined
		}
		return nil, err
	}
	h := syscall.Handle(info.process)
	return func() {
		if h != 0 {
			_, _ = syscall.WaitForSingleObject(h, syscall.INFINITE)
			_ = syscall.CloseHandle(h)
		}
	}, nil
}

type browseInfo struct {
	owner       uintptr
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	lParam      uintptr
	image       int32
}

// pickFolder shows the folder picker; "" when cancelled.
func pickFolder(owner uintptr, title string) string {
	display := make([]uint16, syscall.MAX_PATH)
	t, _ := syscall.UTF16PtrFromString(title)
	bi := browseInfo{owner: owner, displayName: &display[0], title: t, flags: 0x0001 | 0x0040} // RETURNONLYFSDIRS | NEWDIALOGSTYLE
	pidl, _, _ := procSHBrowseFolder.Call(uintptr(unsafe.Pointer(&bi)))                        //nolint:gosec // Win32 call
	if pidl == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(pidl) //nolint:errcheck // frees the item list
	buf := make([]uint16, 32768)
	if r, _, _ := procSHGetPathFromIDL.Call(pidl, uintptr(unsafe.Pointer(&buf[0]))); r == 0 { //nolint:gosec // Win32 call
		return ""
	}
	return syscall.UTF16ToString(buf)
}

type openFileName struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	reserved2     uint32
	flagsEx       uint32
}

// utf16z encodes s (which may contain NULs, as a filter list does) with a
// final NUL.
func utf16z(s string) []uint16 { return append(utf16.Encode([]rune(s)), 0) }

const fileBufLen = 32768

func fileDialog(owner uintptr, save bool, title, defaultName string) string {
	buf := make([]uint16, fileBufLen)
	copy(buf[:fileBufLen-1], utf16.Encode([]rune(defaultName)))
	filter := utf16z("Text files (*.txt)\x00*.txt\x00All files\x00*.*\x00")
	t, _ := syscall.UTF16PtrFromString(title)
	ext, _ := syscall.UTF16PtrFromString("txt")
	ofn := openFileName{owner: owner, filter: &filter[0], file: &buf[0], maxFile: fileBufLen, title: t, defExt: ext}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	proc := procGetOpenFileName
	ofn.flags = 0x00001000 | 0x00000800 // FILEMUSTEXIST | PATHMUSTEXIST
	if save {
		proc = procGetSaveFileName
		ofn.flags = 0x00000002 | 0x00000800 // OVERWRITEPROMPT | PATHMUSTEXIST
	}
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 { //nolint:gosec // Win32 call
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// desktopRestored is Desktop\Restored, the default restore folder.
func desktopRestored() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Desktop", "Restored")
}

var procAppendMenu = user32.NewProc("AppendMenuW")

func appendMenuItem(menu win.HMENU, flags, id uintptr, text string) {
	t, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	procAppendMenu.Call(uintptr(menu), flags, id, uintptr(unsafe.Pointer(t))) //nolint:errcheck,gosec // Win32 call
}

func appendMenu(menu win.HMENU, id uintptr, text string) { appendMenuItem(menu, 0, id, text) } // MF_STRING

func appendSubMenu(menu, sub win.HMENU, text string) {
	appendMenuItem(menu, 0x10, uintptr(sub), text) // MF_POPUP
}

func appendMenuSeparator(menu win.HMENU) { appendMenuItem(menu, 0x800, 0, "") } // MF_SEPARATOR

var (
	procLoadImage        = user32.NewProc("LoadImageW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procGetModuleHandle  = kernel32.NewProc("GetModuleHandleW")
	procSendMessage      = user32.NewProc("SendMessageW")
)

// setWindowIcon gives a window the program icon (resource 1, made by
// scripts/genicon), each in its own size so it stays sharp.
func setWindowIcon(hwnd uintptr) {
	const (
		imageIcon, wmSetIcon           = 1, 0x80
		iconSmall, iconBig             = 0, 1
		smCxIcon, smCyIcon             = 11, 12
		smCxSmIcon, smCySmIcon uintptr = 49, 50
	)
	module, _, _ := procGetModuleHandle.Call(0)
	for _, i := range []struct{ which, cx, cy uintptr }{{iconSmall, smCxSmIcon, smCySmIcon}, {iconBig, smCxIcon, smCyIcon}} {
		w, _, _ := procGetSystemMetrics.Call(i.cx)
		h, _, _ := procGetSystemMetrics.Call(i.cy)
		if icon, _, _ := procLoadImage.Call(module, 1, imageIcon, w, h, 0); icon != 0 {
			procSendMessage.Call(hwnd, wmSetIcon, i.which, icon) //nolint:errcheck,gosec // cosmetic
		}
	}
}
