package store

import "testing"

func TestSearchAcrossPortfolio(t *testing.T) {
	s := testStore(t)
	p, err := s.CreateProject("Warehouse traceability", "Sam", "Alex", "Operations", "Lost batch records", "Trace every batch")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(p.ID, "Map barcode workflow", "Interview the warehouse team", "high", "Taylor", "", 0); err != nil {
		t.Fatal(err)
	}

	results, err := s.Search("barcode", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Kind != "task" || results[0].Link != "/projects/1/board" {
		t.Fatalf("unexpected search results: %+v", results)
	}

	results, err = s.Search("%", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("LIKE wildcard was not escaped: %+v", results)
	}
}
