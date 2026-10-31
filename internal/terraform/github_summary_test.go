package terraform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ansi", "hello world", "hello world"},
		{"color codes", "\x1b[31mred\x1b[0m", "red"},
		{"multiple codes", "\x1b[1;33mwarn\x1b[0m: \x1b[94minfo\x1b[0m", "warn: info"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"under limit", "hello", 10, "hello"},
		{"at limit", "hello", 5, "hello"},
		{"over limit", "hello world", 5, "hello\n\n… (truncated for GitHub Step Summary size)"},
		{"zero max returns as-is", "hello", 0, "hello"},
		{"negative max returns as-is", "hello", -1, "hello"},
		{"unicode", "héllo wörld", 5, "héllo\n\n… (truncated for GitHub Step Summary size)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateRunes(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestCiSummaryCapture(t *testing.T) {
	tests := []struct {
		name       string
		ci         string
		ghSummary  string
		mirrorPath string
		want       bool
	}{
		{"not ci", "", "", "", false},
		{"ci but no paths", "true", "", "", false},
		{"ci with github summary", "true", "/tmp/summary.md", "", true},
		{"ci with mirror path", "true", "", "/tmp/mirror.md", true},
		{"ci case insensitive", "TRUE", "/tmp/summary.md", "", true},
		{"ci with whitespace", " true ", "/tmp/summary.md", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CI", tt.ci)
			t.Setenv("GITHUB_STEP_SUMMARY", tt.ghSummary)
			t.Setenv("TERRAFORMER_SUMMARY_MIRROR_PATH", tt.mirrorPath)
			got := ciSummaryCapture()
			if got != tt.want {
				t.Errorf("ciSummaryCapture() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildGitHubStepSummaryMarkdown(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		md := buildGitHubStepSummaryMarkdown("plan", "services/vpc", "terraform plan", nil, "No changes.", "")
		if !strings.Contains(md, "✅") {
			t.Error("expected success icon")
		}
		if !strings.Contains(md, "**Result:** success") {
			t.Error("expected success result")
		}
		if !strings.Contains(md, "No changes.") {
			t.Error("expected stdout in output")
		}
		if !strings.Contains(md, "`plan`") {
			t.Error("expected command name")
		}
	})

	t.Run("failure", func(t *testing.T) {
		md := buildGitHubStepSummaryMarkdown("apply", "services/vpc", "terraform apply", errors.New("exit 1"), "", "Error: stuff broke")
		if !strings.Contains(md, "❌") {
			t.Error("expected failure icon")
		}
		if !strings.Contains(md, "**Result:** failed") {
			t.Error("expected failed result")
		}
		if !strings.Contains(md, "stuff broke") {
			t.Error("expected stderr in output")
		}
	})

	t.Run("no output", func(t *testing.T) {
		md := buildGitHubStepSummaryMarkdown("plan", "svc", "cmd", nil, "", "")
		if !strings.Contains(md, "No stdout or stderr captured.") {
			t.Error("expected no-output message")
		}
	})

	t.Run("strips ansi from output", func(t *testing.T) {
		md := buildGitHubStepSummaryMarkdown("plan", "svc", "cmd", nil, "\x1b[31mred\x1b[0m", "")
		if strings.Contains(md, "\x1b[") {
			t.Error("expected ANSI codes to be stripped")
		}
		if !strings.Contains(md, "red") {
			t.Error("expected stripped text to remain")
		}
	})
}

func TestAppendGitHubStepSummary(t *testing.T) {
	t.Run("no paths set is no-op", func(t *testing.T) {
		t.Setenv("GITHUB_STEP_SUMMARY", "")
		t.Setenv("TERRAFORMER_SUMMARY_MIRROR_PATH", "")
		err := appendGitHubStepSummary("plan", "svc", "cmd", nil, "out", "err")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("writes to step summary", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "summary.md")
		t.Setenv("GITHUB_STEP_SUMMARY", path)
		t.Setenv("TERRAFORMER_SUMMARY_MIRROR_PATH", "")

		err := appendGitHubStepSummary("plan", "svc", "terraform plan", nil, "output", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "plan") {
			t.Error("expected summary content in file")
		}
	})

	t.Run("writes to both paths", func(t *testing.T) {
		dir := t.TempDir()
		stepPath := filepath.Join(dir, "step.md")
		mirrorPath := filepath.Join(dir, "mirror.md")
		t.Setenv("GITHUB_STEP_SUMMARY", stepPath)
		t.Setenv("TERRAFORMER_SUMMARY_MIRROR_PATH", mirrorPath)

		err := appendGitHubStepSummary("apply", "svc", "terraform apply", nil, "ok", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, p := range []string{stepPath, mirrorPath} {
			data, _ := os.ReadFile(p)
			if !strings.Contains(string(data), "apply") {
				t.Errorf("expected summary content in %s", p)
			}
		}
	})

	t.Run("same path does not duplicate", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "both.md")
		t.Setenv("GITHUB_STEP_SUMMARY", path)
		t.Setenv("TERRAFORMER_SUMMARY_MIRROR_PATH", path)

		err := appendGitHubStepSummary("plan", "svc", "cmd", nil, "out", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, _ := os.ReadFile(path)
		if strings.Count(string(data), "---") != 1 {
			t.Error("expected content written only once when paths are the same")
		}
	})
}
