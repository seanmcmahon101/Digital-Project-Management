package store

import "testing"

func TestCrossProjectRelationshipsAreRejected(t *testing.T) {
	s := testStore(t)
	first, err := s.CreateProject("First project", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateProject("Second project", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.CreateMilestone(second.ID, "Second milestone", "", ""); err != nil {
		t.Fatal(err)
	}
	secondMilestones, err := s.Milestones(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(first.ID, "Task", "", "medium", "", "", secondMilestones[0].ID); err == nil {
		t.Fatal("cross-project task milestone link was accepted")
	}

	requirement, err := s.CreateRequirement(second.ID, "Second requirement", "", "must", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTest(first.ID, requirement.ID, "Test", "", ""); err == nil {
		t.Fatal("cross-project test requirement link was accepted")
	}

	if err := s.CreateRaciActivity(first.ID, "Approve scope"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStakeholder(second.ID, "Taylor", "Sponsor", "high", "high", "supportive", ""); err != nil {
		t.Fatal(err)
	}
	stakeholders, err := s.Stakeholders(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	activities, err := s.RaciActivities(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetRaci(activities[0].ID, stakeholders[0].ID, "A"); err == nil {
		t.Fatal("cross-project RACI assignment was accepted")
	}
}
