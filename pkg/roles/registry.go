package roles

import (
	"github.com/google/uuid"
)

// RoleRegistry holds role UUIDs loaded from the database after seeding.
type RoleRegistry struct {
	UserRoleID  uuid.UUID
	AdminRoleID uuid.UUID
}

// IsAdmin checks if the provided role ID matches the Admin role ID.
func (r *RoleRegistry) IsAdmin(roleID string) bool {
	return roleID == r.AdminRoleID.String()
}

// IsUser checks if the provided role ID matches the User role ID.
func (r *RoleRegistry) IsUser(roleID string) bool {
	return roleID == r.UserRoleID.String()
}
