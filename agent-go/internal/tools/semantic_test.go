package tools

import "testing"

func TestSemanticForOSInfo(t *testing.T) {
	semantic := SemanticForTool("os_info", map[string]any{})

	if semantic.OperationType != "read" {
		t.Fatalf("expected read operation, got %q", semantic.OperationType)
	}
	if semantic.ResourceType != "os_info" {
		t.Fatalf("expected os_info resource type, got %q", semantic.ResourceType)
	}
	if semantic.BoundaryLevel != "public" {
		t.Fatalf("expected public boundary, got %q", semantic.BoundaryLevel)
	}
	if !semantic.AllowedByPolicy {
		t.Fatal("os_info should be allowed by policy")
	}
}

func TestSemanticForPortChecker(t *testing.T) {
	semantic := SemanticForTool("port_checker", map[string]any{
		"host": "127.0.0.1",
		"port": 8080,
	})

	if semantic.OperationType != "inspect" {
		t.Fatalf("expected inspect operation, got %q", semantic.OperationType)
	}
	if semantic.ResourceType != "network_port" {
		t.Fatalf("expected network_port resource type, got %q", semantic.ResourceType)
	}
	if semantic.ResourcePath != "tcp:127.0.0.1:8080" {
		t.Fatalf("unexpected resource path: %q", semantic.ResourcePath)
	}
	if semantic.PermissionScope != "network_port_inspect" {
		t.Fatalf("unexpected permission scope: %q", semantic.PermissionScope)
	}
}

func TestSemanticForSensitiveLogReader(t *testing.T) {
	semantic := SemanticForTool("log_reader", map[string]any{
		"path": "/var/log/secure",
	})

	if semantic.ResourceType != "system_log" {
		t.Fatalf("expected system_log resource type, got %q", semantic.ResourceType)
	}
	if semantic.BoundaryLevel != "sensitive_system_resource" {
		t.Fatalf("expected sensitive boundary, got %q", semantic.BoundaryLevel)
	}
	if !semantic.RequiresPrivilege {
		t.Fatal("sensitive log read should require privilege")
	}
}

func TestSemanticForSSHLoginAnalyzer(t *testing.T) {
	semantic := SemanticForTool("ssh_login_analyzer", map[string]any{})

	if semantic.OperationType != "analyze" {
		t.Fatalf("expected analyze operation, got %q", semantic.OperationType)
	}
	if semantic.ResourceType != "ssh_auth_log" {
		t.Fatalf("expected ssh_auth_log resource type, got %q", semantic.ResourceType)
	}
	if semantic.BoundaryLevel != "sensitive_system_resource" {
		t.Fatalf("expected sensitive boundary, got %q", semantic.BoundaryLevel)
	}
	if !semantic.RequiresPrivilege {
		t.Fatal("ssh auth analysis should require privilege")
	}
	if !semantic.AllowedByPolicy {
		t.Fatal("ssh auth analysis should be allowed by policy")
	}
}

func TestSemanticForAllowedSafeShell(t *testing.T) {
	semantic := SemanticForTool("safe_shell", map[string]any{
		"command": "df -h",
	})

	if !semantic.AllowedByPolicy {
		t.Fatal("df -h should be allowed by policy")
	}
	if semantic.BoundaryLevel != "low" {
		t.Fatalf("expected low boundary, got %q", semantic.BoundaryLevel)
	}
	if semantic.PermissionScope != "safe_command_execute" {
		t.Fatalf("unexpected permission scope: %q", semantic.PermissionScope)
	}
}

func TestSemanticForDangerousSafeShell(t *testing.T) {
	semantic := SemanticForTool("safe_shell", map[string]any{
		"command": "rm -rf /",
	})

	if semantic.AllowedByPolicy {
		t.Fatal("rm -rf / should not be allowed by policy")
	}
	if semantic.BoundaryLevel != "dangerous" {
		t.Fatalf("expected dangerous boundary, got %q", semantic.BoundaryLevel)
	}
	if !semantic.RequiresPrivilege {
		t.Fatal("dangerous shell command should require privilege")
	}
}

func TestSemanticForCompetitionSensingTools(t *testing.T) {
	openFiles := SemanticForTool("open_file_inspector", map[string]any{"path": "/var/log/messages"})
	if openFiles.ResourceType != "open_file_metadata" || openFiles.BoundaryLevel != "sensitive_system_resource" || !openFiles.AllowedByPolicy {
		t.Fatalf("unexpected open-file semantic: %#v", openFiles)
	}
	diskIO := SemanticForTool("disk_io_checker", map[string]any{"sample_ms": 250})
	if diskIO.ResourcePath != "procfs:diskstats" || diskIO.OperationType != "read" {
		t.Fatalf("unexpected disk-I/O semantic: %#v", diskIO)
	}
	zombies := SemanticForTool("process_inspector", map[string]any{"state": "ZOMBIE"})
	if zombies.ResourcePath != "process:all:state=ZOMBIE" {
		t.Fatalf("unexpected zombie semantic path: %#v", zombies)
	}
}
