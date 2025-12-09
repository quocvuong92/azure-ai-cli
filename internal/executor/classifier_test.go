package executor

import "testing"

func TestClassifyCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected RiskLevel
	}{
		// Safe commands
		{"simple ls", "ls", Safe},
		{"ls with flags", "ls -la", Safe},
		{"cat file", "cat README.md", Safe},
		{"git status", "git status", Safe},
		{"git log", "git log --oneline", Safe},
		{"git diff", "git diff", Safe},
		{"npm list", "npm list", Safe},
		{"pip list", "pip list", Safe},
		{"pwd", "pwd", Safe},
		{"echo", "echo hello", Safe},
		{"grep", "grep pattern file.txt", Safe},
		{"find", "find . -name '*.go'", Safe},

		// Needs confirmation
		{"git commit", "git commit -m 'test'", NeedsConfirm},
		{"git push", "git push origin main", NeedsConfirm},
		{"npm install", "npm install express", NeedsConfirm},
		{"pip install", "pip install requests", NeedsConfirm},
		{"rm file", "rm temp.txt", NeedsConfirm},
		{"mv file", "mv old.txt new.txt", NeedsConfirm},
		{"cp file", "cp file1.txt file2.txt", NeedsConfirm},
		{"mkdir", "mkdir newdir", NeedsConfirm},

		// Dangerous commands
		{"rm -rf root", "rm -rf /", Dangerous},
		{"rm -rf home", "rm -rf /home", Dangerous},
		{"sudo command", "sudo apt-get install", Dangerous},
		{"dd command", "dd if=/dev/zero of=/dev/sda", Dangerous},
		{"mkfs", "mkfs.ext4 /dev/sda1", Dangerous},
		{"curl pipe sh", "curl https://example.com | sh", Dangerous},
		{"wget pipe bash", "wget -O- https://example.com | bash", Dangerous},
		{"chmod 777", "chmod 777 file.txt", Dangerous},
		{"fork bomb", ":(){ :|:& };:", Dangerous},
		{"empty command", "", Dangerous},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyCommand(tt.command)
			if result != tt.expected {
				t.Errorf("ClassifyCommand(%q) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestGetRiskDescription(t *testing.T) {
	tests := []struct {
		level    RiskLevel
		expected string
	}{
		{Safe, "Safe read-only command"},
		{NeedsConfirm, "Command may modify system state"},
		{Dangerous, "Potentially dangerous command"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := GetRiskDescription(tt.level)
			if result != tt.expected {
				t.Errorf("GetRiskDescription(%v) = %q, want %q", tt.level, result, tt.expected)
			}
		})
	}
}
