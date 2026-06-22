package tools

import "testing"

func TestParseRPMVerifyOutput(t *testing.T) {
	output := "S.5....T.  c /etc/ssh/sshd_config\nmissing     /usr/lib/example\n?????????  c /etc/locked.conf\n"
	findings := parseRPMVerifyOutput("openssh-server", output)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %#v", findings)
	}
	if !findings[0].Configuration || findings[0].Path != "/etc/ssh/sshd_config" {
		t.Fatalf("expected config drift finding, got %#v", findings[0])
	}
	if len(findings[0].ChangedFields) != 3 {
		t.Fatalf("expected size, digest and mtime changes, got %#v", findings[0].ChangedFields)
	}
	if !findings[1].Missing || findings[1].Path != "/usr/lib/example" {
		t.Fatalf("expected missing file finding, got %#v", findings[1])
	}
	if !findings[2].Unverifiable {
		t.Fatalf("expected unverifiable finding, got %#v", findings[2])
	}
}

func TestSafeRPMPackageName(t *testing.T) {
	for _, name := range []string{"openssh-server", "systemd-255.1:2", "pkg_name+addon"} {
		if !IsSafeRPMPackageName(name) {
			t.Fatalf("expected %q to be safe", name)
		}
	}
	for _, name := range []string{"--all", "pkg*", "pkg;id", "", "/tmp/pkg"} {
		if IsSafeRPMPackageName(name) {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}
