// Unit test for role ranking — pure logic, no database.
//
// Course mapping: Chapter 38 — unit tests. RoleRank is what RBAC compares
// against, so its ordering (owner > admin > member > unknown) must hold.
package orgs

import "testing"

func TestRoleRankOrdering(t *testing.T) {
	if !(RoleRank(RoleOwner) > RoleRank(RoleAdmin)) {
		t.Error("owner should outrank admin")
	}
	if !(RoleRank(RoleAdmin) > RoleRank(RoleMember)) {
		t.Error("admin should outrank member")
	}
	if !(RoleRank(RoleMember) > RoleRank("nonsense")) {
		t.Error("member should outrank an unknown role")
	}
	if RoleRank("nonsense") != 0 {
		t.Errorf("unknown role rank = %d, want 0", RoleRank("nonsense"))
	}
}

func TestRoleRankExactValues(t *testing.T) {
	tests := map[string]int{
		RoleOwner:  3,
		RoleAdmin:  2,
		RoleMember: 1,
		"":         0,
		"ghost":    0,
	}
	for role, want := range tests {
		if got := RoleRank(role); got != want {
			t.Errorf("RoleRank(%q) = %d, want %d", role, got, want)
		}
	}
}
