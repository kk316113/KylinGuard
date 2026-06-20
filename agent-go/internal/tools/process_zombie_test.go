package tools

import "testing"

func TestParsePSAuxFindsAndFiltersZombieProcesses(t *testing.T) {
	fixture := "USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND\n" +
		"root 1 0.0 0.1 1000 100 ? Ss 10:00 0:01 /sbin/init\n" +
		"app 42 0.0 0.0 0 0 ? Z 10:01 0:00 [worker] <defunct>\n" +
		"app 43 0.0 0.0 0 0 ? Z+ 10:01 0:00 [worker2] <defunct>\n" +
		"app 44 1.0 0.5 1000 100 ? R 10:02 0:02 worker3\n"
	processes, zombies := parsePSAux(fixture, "", "ZOMBIE", 20)
	if zombies != 2 || len(processes) != 2 {
		t.Fatalf("expected 2 zombies, got count=%d processes=%#v", zombies, processes)
	}
	if processes[0].PID != 42 || processes[1].State != "Z+" {
		t.Fatalf("unexpected zombie records: %#v", processes)
	}
}

func TestProcessRiskLevelUsesZombieCount(t *testing.T) {
	if processRiskLevel(0) != "low" || processRiskLevel(1) != "medium" || processRiskLevel(5) != "high" {
		t.Fatal("unexpected zombie risk thresholds")
	}
}

func TestMatchesProcessState(t *testing.T) {
	tests := []struct {
		state  string
		filter string
		want   bool
	}{
		{"R+", "RUNNING", true},
		{"Ss", "SLEEPING", true},
		{"I<", "SLEEPING", true},
		{"Z", "ZOMBIE", true},
		{"T", "STOPPED", true},
		{"S", "ZOMBIE", false},
	}
	for _, test := range tests {
		if got := matchesProcessState(test.state, test.filter); got != test.want {
			t.Fatalf("state=%s filter=%s got=%v want=%v", test.state, test.filter, got, test.want)
		}
	}
}
