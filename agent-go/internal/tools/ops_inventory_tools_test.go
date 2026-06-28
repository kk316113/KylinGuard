package tools

import "testing"

func TestParseSystemctlListUnits(t *testing.T) {
	units := parseSystemctlListUnits("sshd.service loaded active running OpenSSH server daemon\nfirewalld.service loaded inactive dead firewalld firewall daemon\n", 10)
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}
	if units[0].Unit != "sshd.service" || units[0].Active != "active" || units[0].Sub != "running" {
		t.Fatalf("unexpected first unit: %#v", units[0])
	}
}

func TestParseLSBLKJSON(t *testing.T) {
	devices, err := parseLSBLKJSON(`{"blockdevices":[{"name":"sda","type":"disk","size":"40G","fstype":"","mountpoint":"","rota":true,"model":"Virtual Disk"}]}`, 10)
	if err != nil {
		t.Fatalf("parseLSBLKJSON failed: %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "sda" || devices[0].Type != "disk" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestParseFindmntJSON(t *testing.T) {
	mounts, err := parseFindmntJSON(`{"filesystems":[{"target":"/","source":"/dev/sda1","fstype":"xfs","options":"rw,relatime"}]}`, 10)
	if err != nil {
		t.Fatalf("parseFindmntJSON failed: %v", err)
	}
	if len(mounts) != 1 || mounts[0].Target != "/" || mounts[0].FSType != "xfs" {
		t.Fatalf("unexpected mounts: %#v", mounts)
	}
}

func TestParseRPMPackageQuery(t *testing.T) {
	packages := parseRPMPackageQuery("openssh-server\t9.3\t1.el9\tx86_64\nkernel\t5.14\t1.el9\tx86_64\n", "ssh", 10)
	if len(packages) != 1 {
		t.Fatalf("expected one package, got %#v", packages)
	}
	if packages[0].Name != "openssh-server" || packages[0].Arch != "x86_64" {
		t.Fatalf("unexpected package: %#v", packages[0])
	}
}
