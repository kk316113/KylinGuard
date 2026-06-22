package tools

import "testing"

func TestAllowedOpenFilePaths(t *testing.T) {
	for _, path := range []string{
		"/var/log/messages",
		"/var/log/audit/audit.log",
		"/tmp/demo.log",
		"/var/tmp/app.trace",
		"/opt/kylin-guard/logs/agent.log",
	} {
		if !IsAllowedOpenFilePath(path) {
			t.Fatalf("expected allowed open-file path %q", path)
		}
	}
	for _, path := range []string{
		"/etc/shadow",
		"/root/.ssh/id_rsa",
		"/home/user/secret",
		"/proc/1/mem",
		"relative.log",
		"/var/log/../../etc/shadow",
	} {
		if IsAllowedOpenFilePath(path) {
			t.Fatalf("expected denied open-file path %q", path)
		}
	}
}

func TestParseLsofFieldOutput(t *testing.T) {
	output := "p123\ncpostgres\nu26\nf5w\ntREG\nn/var/log/postgresql/server.log\n" +
		"f6r\ntREG\nn/var/log/postgresql/query.log\n" +
		"p456\ncsshd\nu0\nf3r\ntREG\nn/var/log/secure\n"
	files := parseLsofFieldOutput(output, 10)
	if len(files) != 3 {
		t.Fatalf("expected 3 records, got %#v", files)
	}
	if files[0].PID != 123 || files[0].Command != "postgres" || files[0].Name != "/var/log/postgresql/server.log" {
		t.Fatalf("unexpected first record: %#v", files[0])
	}
	if files[2].PID != 456 || files[2].FD != "3r" {
		t.Fatalf("unexpected final record: %#v", files[2])
	}
}

func TestParseLsofFieldOutputHonorsLimit(t *testing.T) {
	output := "p1\nctest\nu1000\nf1r\ntREG\nn/tmp/a\nf2r\ntREG\nn/tmp/b\n"
	files := parseLsofFieldOutput(output, 1)
	if len(files) != 1 || files[0].Name != "/tmp/a" {
		t.Fatalf("unexpected limited records: %#v", files)
	}
}
