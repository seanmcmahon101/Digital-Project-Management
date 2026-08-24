package store

import "testing"

func TestProjectDocumentsLifecycleAndLinkValidation(t *testing.T) {
	s := testStore(t)
	p, err := s.CreateProject("Documented project", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddFileDocument(p.ID, "Design", "Approved copy", "design.pdf", "opaque.pdf", "application/pdf", 1234); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLinkDocument(p.ID, "SharePoint folder", "Authoritative source", "https://example.com/documents/folder"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLinkDocument(p.ID, "Unsafe", "", "javascript:alert(1)"); err == nil {
		t.Fatal("unsafe link scheme should be rejected")
	}
	docs, err := s.Documents(p.ID)
	if err != nil || len(docs) != 2 {
		t.Fatalf("documents = %d, err=%v", len(docs), err)
	}
	if docs[0].Kind != "link" || docs[1].OriginalName != "design.pdf" {
		t.Fatalf("unexpected documents: %+v", docs)
	}
	if err := s.DeleteDocument(docs[1].ID); err != nil {
		t.Fatal(err)
	}
	docs, _ = s.Documents(p.ID)
	if len(docs) != 1 {
		t.Fatalf("want 1 document after delete, got %d", len(docs))
	}
}

func TestImportProjectRowsCreatesUpdatesAndAdvancesCounter(t *testing.T) {
	s := testStore(t)
	created, updated, err := s.ImportProjectRows([]ProjectTransferRow{{
		Code: "DPM-010", Name: "Imported", Stage: "plan", Status: "active",
		Sponsor: "Sponsor", TargetEnd: "2027-01-31",
	}})
	if err != nil || created != 1 || updated != 0 {
		t.Fatalf("first import: created=%d updated=%d err=%v", created, updated, err)
	}
	created, updated, err = s.ImportProjectRows([]ProjectTransferRow{{
		Code: "DPM-010", Name: "Imported revised", Stage: "build", Status: "on_hold",
	}})
	if err != nil || created != 0 || updated != 1 {
		t.Fatalf("second import: created=%d updated=%d err=%v", created, updated, err)
	}
	projects, listErr := s.Projects()
	if listErr != nil || len(projects) != 1 {
		t.Fatalf("list projects: %v (%d)", listErr, len(projects))
	}
	p := projects[0]
	if p.Name != "Imported revised" || p.Stage != "build" || p.Status != "on_hold" {
		t.Fatalf("updated project: %+v", p)
	}
	next, err := s.CreateProject("Next", "", "", "", "", "")
	if err != nil || next.Code != "DPM-011" {
		t.Fatalf("counter not advanced: %+v err=%v", next, err)
	}
	if _, _, err := s.ImportProjectRows([]ProjectTransferRow{{Code: "BAD CODE", Name: "No", Stage: "bogus"}}); err == nil {
		t.Fatal("invalid rows should be rejected")
	}
}
