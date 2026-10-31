package terraform

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const maxStepSummaryRunes = 120_000

// EnvSummaryMirrorPath names an extra file that receives the same markdown as
// GITHUB_STEP_SUMMARY (useful for PR comments or non-GitHub CI systems).
const EnvSummaryMirrorPath = "TERRAFORMER_SUMMARY_MIRROR_PATH"

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func ciSummaryCapture() bool {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
		return false
	}
	return githubStepSummaryPath() != "" || summaryMirrorPath() != ""
}

func githubStepSummaryPath() string {
	return strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY"))
}

func summaryMirrorPath() string {
	return strings.TrimSpace(os.Getenv(EnvSummaryMirrorPath))
}

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "\n\n… (truncated for GitHub Step Summary size)"
}

func buildGitHubStepSummaryMarkdown(cmdName, serviceName, fullCmd string, runErr error, stdout, stderr string) string {
	status := "failed"
	statusIcon := "❌"
	if runErr == nil {
		status = "success"
		statusIcon = "✅"
	}

	out := truncateRunes(stripANSI(stdout), maxStepSummaryRunes)
	errOut := truncateRunes(stripANSI(stderr), maxStepSummaryRunes)

	var b strings.Builder
	fmt.Fprintf(&b, "### %s `%s` — `%s`\n\n", statusIcon, cmdName, serviceName)
	fmt.Fprintf(&b, "**Command:** `%s`\n\n", fullCmd)
	fmt.Fprintf(&b, "**Result:** %s\n\n", status)

	writeBlock := func(title, body string) {
		if body == "" {
			return
		}
		fmt.Fprintf(&b, "<details><summary>%s</summary>\n\n```text\n", title)
		b.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n\n</details>\n\n")
	}
	writeBlock("Stdout", out)
	writeBlock("Stderr", errOut)
	if out == "" && errOut == "" {
		b.WriteString("_No stdout or stderr captured._\n\n")
	}
	b.WriteString("---\n")
	return b.String()
}

func appendMarkdownToFile(path, md string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(md)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

// appendGitHubStepSummary appends a markdown section for this terraform
// invocation to GITHUB_STEP_SUMMARY and/or TERRAFORMER_SUMMARY_MIRROR_PATH.
func appendGitHubStepSummary(cmdName, serviceName, fullCmd string, runErr error, stdout, stderr string) error {
	stepPath := githubStepSummaryPath()
	mirrorPath := summaryMirrorPath()
	if stepPath == "" && mirrorPath == "" {
		return nil
	}

	md := buildGitHubStepSummaryMarkdown(cmdName, serviceName, fullCmd, runErr, stdout, stderr)

	if stepPath != "" {
		if err := appendMarkdownToFile(stepPath, md); err != nil {
			return err
		}
	}
	if mirrorPath != "" && mirrorPath != stepPath {
		if err := appendMarkdownToFile(mirrorPath, md); err != nil {
			return err
		}
	}
	return nil
}
