package service_test

import (
	"testing"
)

func TestUserService_List(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.addUser(t, "zoe")
	f.addUser(t, "Ada")
	f.addUser(t, "bob")

	users, err := f.users.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, u := range users {
		names = append(names, u.DisplayName)
	}

	if len(names) != 3 || names[0] != "Ada" || names[1] != "bob" || names[2] != "zoe" {
		t.Fatalf("List() names = %v, want [Ada bob zoe]", names)
	}
}
