package terraform

import (
	"bytes"
	"testing"
)

func TestPlanFilter_passesMarkerLine(t *testing.T) {
	var out bytes.Buffer
	pf := &PlanFilter{Out: &out}

	pf.Write([]byte("Initializing modules...\nDownloading...\n"))
	if out.Len() != 0 {
		t.Fatalf("expected no output before marker, got %q", out.String())
	}

	pf.Write([]byte("Terraform used the selected providers to generate the following plan.\n"))
	if out.Len() == 0 {
		t.Fatal("expected output after marker")
	}
	if !bytes.Contains(out.Bytes(), []byte("Terraform used the selected providers")) {
		t.Errorf("output should contain marker text, got %q", out.String())
	}
}

func TestPlanFilter_noChanges(t *testing.T) {
	var out bytes.Buffer
	pf := &PlanFilter{Out: &out}

	pf.Write([]byte("Refreshing state...\n"))
	pf.Write([]byte("No changes. Your infrastructure matches the configuration.\n"))
	if !bytes.Contains(out.Bytes(), []byte("No changes.")) {
		t.Errorf("expected 'No changes.' in output, got %q", out.String())
	}
}

func TestPlanFilter_alreadyPassing(t *testing.T) {
	var out bytes.Buffer
	pf := &PlanFilter{Out: &out}

	pf.Write([]byte("No changes.\n"))
	out.Reset()

	pf.Write([]byte("more output after marker\n"))
	if !bytes.Contains(out.Bytes(), []byte("more output after marker")) {
		t.Errorf("expected passthrough after marker triggered, got %q", out.String())
	}
}

func TestPlanFilter_contextLines(t *testing.T) {
	var out bytes.Buffer
	pf := &PlanFilter{Out: &out}

	pf.Write([]byte("line1\nvar.region\n  Region for deployment\n\nEnter a value: "))
	got := out.String()
	if !bytes.Contains([]byte(got), []byte("var.region")) {
		t.Errorf("expected context lines before 'Enter a value:', got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Enter a value:")) {
		t.Errorf("expected marker text in output, got %q", got)
	}
}

func TestPlanFilter_Flush(t *testing.T) {
	var out bytes.Buffer
	pf := &PlanFilter{Out: &out}

	pf.Write([]byte("unexpected format with no markers\n"))
	if out.Len() != 0 {
		t.Fatal("expected no output before Flush")
	}

	pf.Flush()
	if !bytes.Contains(out.Bytes(), []byte("unexpected format")) {
		t.Errorf("expected Flush to write buffered content, got %q", out.String())
	}
}

func TestPlanFilter_FlushNoopWhenPassing(t *testing.T) {
	var out bytes.Buffer
	pf := &PlanFilter{Out: &out}

	pf.Write([]byte("No changes.\n"))
	before := out.String()

	pf.Flush()
	if out.String() != before {
		t.Error("Flush should be a no-op when already passing")
	}
}
