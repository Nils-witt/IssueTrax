package domain_test

import (
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestAuditChanges(t *testing.T) {
	t.Parallel()

	c := domain.AuditChanges{}
	c.Set("same", "a", "a")
	c.Set("title", "old", "new")
	c.Text("description", "secret", "other secret")
	c.Text("body", "x", "x")
	c.List("tags", []string{"b", "a"}, []string{"a", "b"})
	c.List("assignees", []string{"bob"}, []string{"cy", "ada"})

	for _, field := range []string{"same", "body", "tags"} {
		if _, ok := c[field]; ok {
			t.Errorf("unchanged field %q recorded: %+v", field, c[field])
		}
	}

	if v := c["title"]; *v.Old != "old" || *v.New != "new" || v.Redacted {
		t.Errorf("title = %+v", v)
	}

	if v := c["assignees"]; *v.Old != "bob" || *v.New != "ada, cy" {
		t.Errorf("assignees = %+v", v)
	}

	if v := c["description"]; !v.Redacted || v.Old != nil || v.New != nil {
		t.Errorf("description = %+v, want redacted", v)
	}
}

func TestAuditChanges_Initial(t *testing.T) {
	t.Parallel()

	c := domain.AuditChanges{}
	c.Initial("empty", "")
	c.Initial("key", "TRAX")
	c.InitialText("note", "hidden")
	c.InitialText("blank", "")
	c.InitialList("labels", nil)
	c.InitialList("tags", []string{"ui", "bug"})

	if len(c) != 3 {
		t.Fatalf("changes = %+v, want key, note and tags only", c)
	}

	if v := c["key"]; v.Old != nil || *v.New != "TRAX" {
		t.Errorf("key = %+v", v)
	}

	if v := c["tags"]; v.Old != nil || *v.New != "bug, ui" {
		t.Errorf("tags = %+v", v)
	}

	if v := c["note"]; !v.Redacted || v.New != nil {
		t.Errorf("note = %+v, want redacted", v)
	}
}

func TestAuditAction_Valid(t *testing.T) {
	t.Parallel()

	if !domain.AuditIssueUpdate.Valid() || domain.AuditAction("issue.delete").Valid() {
		t.Fatal("Valid() misclassifies actions")
	}
}
