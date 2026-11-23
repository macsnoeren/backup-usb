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

package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/msg"
)

// RenderReport formats the report as text for report.txt.
func RenderReport(r *RunReport, lang msg.Lang) string {
	p := msg.Printer{Lang: lang}
	var b strings.Builder
	line := func(key string, v any) { fmt.Fprintf(&b, "%-14s %v\n", p.T(key)+":", v) }
	b.WriteString(p.T("report.title") + "\n\n")
	line("report.status", p.T("status."+r.Status))
	line("report.disk", r.Disk)
	line("report.period", r.Period)
	line("report.folder", r.Folder)
	line("report.started", r.Started.Format(time.RFC3339))
	line("report.finished", r.Finished.Format(time.RFC3339))
	enc := p.T("report.no")
	if r.Encrypted {
		enc = p.T("report.yes")
	}
	line("report.encrypted", enc)
	if len(r.Keys) > 0 {
		line("report.keys", strings.Join(r.Keys, ", "))
	}
	if r.Health != "" {
		line("report.health", r.Health)
	}
	if r.FreeBytes > 0 {
		line("report.free", humanBytes(r.FreeBytes))
	}
	if r.FolderCount > 0 {
		line("report.folders", r.FolderCount)
	}
	if len(r.Rotated) > 0 {
		line("report.rotated", strings.Join(r.Rotated, ", "))
	}
	if r.Error != "" {
		line("report.error", r.Error)
	}
	for _, s := range r.Sources {
		fmt.Fprintf(&b, "\n[%s] %s\n", s.Tag, p.T("status."+s.Status))
		if s.Archive != "" && s.Status != StatusEmpty {
			fmt.Fprintf(&b, "  %-12s %s\n", p.T("report.archive")+":", s.Archive)
		}
		if s.Status == StatusOK || s.Status == StatusWarning {
			fmt.Fprintf(&b, "  %-12s %d\n", p.T("report.files")+":", s.Files)
			fmt.Fprintf(&b, "  %-12s %s\n", p.T("report.size")+":", humanBytes(s.Bytes))
			fmt.Fprintf(&b, "  %-12s %s\n", p.T("report.zipSize")+":", humanBytes(uint64(max(s.ZipBytes, 0))))
			fmt.Fprintf(&b, "  %-12s %s\n", "SHA-256:", s.SHA256)
			if s.Verified {
				fmt.Fprintf(&b, "  %-12s %s\n", p.T("report.verify")+":", "OK")
			}
		}
		if s.SkippedCount > 0 {
			fmt.Fprintf(&b, "  %-12s %d\n", p.T("report.skipped")+":", s.SkippedCount)
			for i, sk := range s.Skipped {
				if i == 10 {
					fmt.Fprintf(&b, "    …\n")
					break
				}
				fmt.Fprintf(&b, "    %s (%s)\n", sk.Path, sk.Reason)
			}
		}
		if s.WarningCount > 0 {
			fmt.Fprintf(&b, "  %-12s %d\n", p.T("report.warnings")+":", s.WarningCount)
			for i, w := range s.Warnings {
				if i == 10 {
					fmt.Fprintf(&b, "    …\n")
					break
				}
				fmt.Fprintf(&b, "    %s (%s)\n", w.Path, w.Reason)
			}
		}
		if s.Error != "" {
			fmt.Fprintf(&b, "  %-12s %s\n", p.T("report.error")+":", s.Error)
		}
	}
	if len(r.Messages) > 0 {
		b.WriteString("\n")
		for _, m := range r.Messages {
			b.WriteString(m + "\n")
		}
	}
	return b.String()
}
