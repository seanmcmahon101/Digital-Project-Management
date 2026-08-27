package web

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

func TestProjectCSVAndXLSXRoundTrip(t *testing.T) {
	want := []store.ProjectTransferRow{{
		Code: "DPM-007", Name: "Unicode – café", Stage: "define", Status: "active",
		ProblemStatement: "Line one\nLine two, with comma", ScopeIn: "A & B", TargetEnd: "2027-02-03",
	}}
	var csvOut bytes.Buffer
	if err := writeProjectsCSV(&csvOut, want); err != nil {
		t.Fatal(err)
	}
	gotCSV, err := readProjectsCSV(bytes.NewReader(csvOut.Bytes()))
	if err != nil || len(gotCSV) != 1 || gotCSV[0].Name != want[0].Name || gotCSV[0].ProblemStatement != want[0].ProblemStatement {
		t.Fatalf("CSV round trip: %+v err=%v", gotCSV, err)
	}

	var xlsxOut bytes.Buffer
	if err := writeProjectsXLSX(&xlsxOut, want); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(xlsxOut.Bytes(), []byte("PK")) {
		t.Fatal("XLSX is not a ZIP workbook")
	}
	gotXLSX, err := readProjectsXLSX(xlsxOut.Bytes())
	if err != nil || len(gotXLSX) != 1 || gotXLSX[0].Name != want[0].Name || gotXLSX[0].ScopeIn != "A & B" {
		t.Fatalf("XLSX round trip: %+v err=%v", gotXLSX, err)
	}
}

func TestProjectCSVRequiresNameColumn(t *testing.T) {
	var b bytes.Buffer
	cw := csv.NewWriter(&b)
	_ = cw.Write([]string{"code", "stage"})
	_ = cw.Write([]string{"DPM-001", "intake"})
	cw.Flush()
	if _, err := readProjectsCSV(&b); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected missing-name-column error, got %v", err)
	}
}

func TestProjectCSVNeutralisesSpreadsheetFormulas(t *testing.T) {
	rows := []store.ProjectTransferRow{{
		Code: "DPM-001", Name: "=HYPERLINK(\"https://example.invalid\")",
		Stage: "intake", Status: "active", Sponsor: "+cmd|' /C calc'!A0",
		Lead: "@SUM(1+1)", Department: "-2+3", Goal: "ordinary text",
	}}
	var out bytes.Buffer
	if err := writeProjectsCSV(&out, rows); err != nil {
		t.Fatal(err)
	}
	cr := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(out.Bytes(), []byte("\xef\xbb\xbf"))))
	table, err := cr.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != 2 {
		t.Fatalf("CSV rows = %d, want 2", len(table))
	}
	for _, index := range []int{1, 4, 5, 6} {
		if !strings.HasPrefix(table[1][index], "'") {
			t.Errorf("cell %d was not neutralised: %q", index, table[1][index])
		}
	}
	if table[1][8] != "ordinary text" {
		t.Fatalf("ordinary text changed: %q", table[1][8])
	}
}

func TestSafeUploadPathRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	if _, ok := safeUploadPath(base, `..\secret.db`); ok {
		t.Fatal("Windows traversal accepted")
	}
	if _, ok := safeUploadPath(base, "../secret.db"); ok {
		t.Fatal("slash traversal accepted")
	}
	path, ok := safeUploadPath(base, "0123456789abcdef.pdf")
	if !ok || !strings.HasPrefix(path, base) {
		t.Fatalf("safe path rejected: %q", path)
	}
}
