package fsutil

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

var virtualFSTypes = map[string]struct{}{
	"proc": {}, "sysfs": {}, "devtmpfs": {}, "devpts": {}, "tmpfs": {},
	"cgroup": {}, "cgroup2": {}, "pstore": {}, "bpf": {}, "tracefs": {},
	"debugfs": {}, "securityfs": {}, "hugetlbfs": {}, "mqueue": {},
	"configfs": {}, "fusectl": {}, "rpc_pipefs": {}, "binfmt_misc": {},
	"autofs": {}, "overlay": {}, "squashfs": {}, "nsfs": {}, "ramfs": {},
	"efivarfs": {}, "fuse.gvfsd-fuse": {}, "fuse.portal": {},
}

var networkFSTypes = map[string]struct{}{
	"nfs": {}, "nfs4": {}, "cifs": {}, "smb3": {}, "smb": {},
	"fuse.sshfs": {}, "fuse.rclone": {}, "glusterfs": {}, "ceph": {},
	"afs": {},
}

// systemMountPoints are never shown as "Disks" (covered by Places / OS).
var systemMountPoints = map[string]struct{}{
	"/": {}, "/boot": {}, "/boot/efi": {}, "/home": {}, "/usr": {},
	"/var": {}, "/tmp": {}, "/opt": {}, "/srv": {}, "/etc": {},
	"/dev": {}, "/proc": {}, "/sys": {}, "/run": {}, "/root": {},
}

// ListMountedDisks returns real disk / network volume mounts for the Places sidebar.
// Bind mounts (Docker volume mounts, file binds, etc.) are excluded. Returns nil
// when nothing qualifies — the UI hides the Disks section in that case.
func ListMountedDisks() []Root {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	defer f.Close()

	seen := map[string]struct{}{}
	out := make([]Root, 0, 8)
	sc := bufio.NewScanner(f)
	// mountinfo lines can be long (overlay options).
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		root, mount, fstype, source, ok := parseMountInfo(sc.Text())
		if !ok {
			continue
		}
		if _, skip := virtualFSTypes[fstype]; skip {
			continue
		}
		if strings.HasPrefix(fstype, "fuse.") && fstype != "fuseblk" {
			if _, net := networkFSTypes[fstype]; !net {
				// Keep only FUSE mounts that look like user volumes.
				if !isUserVolumePath(mount) {
					continue
				}
			}
		}

		mount = filepath.Clean(unescapeMount(mount))
		root = filepath.Clean(unescapeMount(root))
		source = unescapeMount(source)

		if !filepath.IsAbs(mount) {
			continue
		}
		if _, sys := systemMountPoints[mount]; sys {
			continue
		}
		// Skip anything under system trees that is not a media path.
		if !isUserVolumePath(mount) && isSystemSubtree(mount) {
			continue
		}

		// Bind mounts of a subdirectory/file have a non-root "root" field.
		// Real block/network volumes mount the FS root ("/") — btrfs subvols use "/@…".
		if !isFilesystemRoot(root, fstype) {
			continue
		}

		_, isNetwork := networkFSTypes[fstype]
		isBlock := strings.HasPrefix(source, "/dev/")
		isMedia := isUserVolumePath(mount)
		if !isMedia && !isBlock && !isNetwork {
			continue
		}
		// Extra block-device mounts outside media dirs: only top-level paths
		// (e.g. /data, /storage1) — not deep binds that slipped through.
		if !isMedia && strings.Count(mount, "/") > 1 {
			continue
		}

		// Must be a directory (skip file binds like /etc/resolv.conf).
		fi, err := os.Stat(mount)
		if err != nil || !fi.IsDir() {
			continue
		}

		if _, ok := seen[mount]; ok {
			continue
		}
		seen[mount] = struct{}{}

		label := filepath.Base(mount)
		if label == "/" || label == "." || label == "" {
			label = mount
		}
		out = append(out, Root{
			Path:  mount,
			Label: label,
			Icon:  "HardDrive",
			Group: "disks",
		})
	}
	return out
}

func parseMountInfo(line string) (root, mount, fstype, source string, ok bool) {
	// Format: … root mountpoint mountopts [optional…] - fstype source superopts
	sep := strings.Index(line, " - ")
	if sep < 0 {
		return "", "", "", "", false
	}
	left := strings.Fields(line[:sep])
	right := strings.Fields(line[sep+3:])
	// left: id parent major:minor root mountpoint opts [opt]*
	if len(left) < 6 || len(right) < 2 {
		return "", "", "", "", false
	}
	return left[3], left[4], right[0], right[1], true
}

func isFilesystemRoot(root, fstype string) bool {
	if root == "/" {
		return true
	}
	// btrfs subvolume mounts use paths like /@ or /@home.
	if fstype == "btrfs" && strings.HasPrefix(root, "/@") {
		return true
	}
	return false
}

func isUserVolumePath(mount string) bool {
	return strings.HasPrefix(mount, "/mnt/") ||
		mount == "/mnt" ||
		strings.HasPrefix(mount, "/media/") ||
		mount == "/media" ||
		strings.HasPrefix(mount, "/run/media/")
}

func isSystemSubtree(mount string) bool {
	for _, p := range []string{
		"/proc/", "/sys/", "/dev/", "/run/", "/var/lib/docker/",
		"/var/lib/containerd/", "/snap/",
	} {
		if strings.HasPrefix(mount, p) {
			return true
		}
	}
	return false
}

func unescapeMount(s string) string {
	replacer := strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	)
	return replacer.Replace(s)
}
