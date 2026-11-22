package cli

import (
	"fmt"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func init() {
	register(&command{name: "disks", summary: "Show registered and connected disks side by side.", run: cmdDisks})
	register(&command{name: "forget", summary: "Remove the registration of a disk (vault password).", run: cmdForget})
}

func cmdDisks(c *ctx, args []string) error {
	fs := c.newFlags("disks")
	asJSON := fs.Bool("json", false, "print as JSON")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	ctx, cancel := c.context()
	defer cancel()
	rep, err := app.Disks(ctx, c.deps(cfg), cfg)
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, rep)
	}
	w := c.env.Stdout
	fmt.Fprintln(w, c.p.T("disk.registered"))
	if len(rep.Registered) == 0 {
		fmt.Fprintln(w, c.p.T("disk.none"))
	}
	for _, r := range rep.Registered {
		where := c.p.T("disk.notConnected")
		if r.Connected {
			where = c.p.T("disk.isConnected", r.Mount)
		}
		fmt.Fprintf(w, "  %-10s %-5s %s  vol %s  part %s  hw %s  %s\n", r.Name, r.Weeks, r.Enrolled, r.VolumeSerial, r.PartitionID, orDash(r.HardwareSerial), where)
	}
	fmt.Fprintln(w, c.p.T("disk.connected"))
	if len(rep.Connected) == 0 {
		fmt.Fprintln(w, c.p.T("disk.none"))
	}
	for _, v := range rep.Connected {
		state := c.p.T("disk.state."+v.State, v.Name)
		fmt.Fprintf(w, "  %-12s %-10s %-6s vol %s  part %s  hw %s  %s\n", orDash(v.Mount), orDash(v.Label), v.FileSystem, v.VolumeSerial, v.PartitionID, orDash(v.HardwareSerial), state)
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cmdForget(c *ctx, args []string) error {
	fs := c.newFlags("forget")
	stdin := fs.Bool("password-stdin", false, "read the vault password from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageErr("usage: forget <name>")
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	key, err := c.operatorKey(cfg, *stdin, *vaultPath)
	if err != nil {
		return err
	}
	defer key.Wipe()
	if err := app.Forget(c.deps(cfg), cfg, pos[0], key); err != nil {
		return err
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("disk.forgotten", pos[0]))
	return nil
}
