package user

import (
	"database/sql/driver"
	"time"

	"geeks-accelerator/oss/saas-starter-kit/example-project/internal/platform/auth"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"gopkg.in/go-playground/validator.v9"
)

// UserAccount defines the one to many relationship of an user to an account. This
// will enable a single user access to multiple accounts without having duplicate
// users. Each association of a user to an account has a set of roles and a status
// defined for the user. The roles will be applied to enforce ACLs across the
// application. The status will allow users to be managed on by account with users
// being global to the application.
type UserAccount struct {
	ID         string            `json:"id"`
	UserID     string            `json:"user_id"`
	AccountID  string            `json:"account_id"`
	Roles      UserAccountRoles  `json:"roles"`
	Status     UserAccountStatus `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	ArchivedAt pq.NullTime       `json:"archived_at"`
}

// CreateUserAccountRequest defines the information is needed to associate a user to an
// account. Users are global to the application and each users access can be managed
// on an account level. If a current entry exists in the database but is archived,
// it will be un-archived.
type CreateUserAccountRequest struct {
	UserID    string             `validate:"required,uuid"`
	AccountID string             `validate:"required,uuid"`
	Roles     UserAccountRoles   `json:"roles" validate:"required,dive,oneof=admin user"`
	Status    *UserAccountStatus `json:"status" validate:"omitempty,oneof=active invited disabled"`
}

// UpdateUserAccountRequest defines the information needed to update the roles or the
// status for an existing user account.
type UpdateUserAccountRequest struct {
	UserID    string             `validate:"required,uuid"`
	AccountID string             `validate:"required,uuid"`
	Roles     *UserAccountRoles  `json:"roles" validate:"required,dive,oneof=admin user"`
	Status    *UserAccountStatus `json:"status" validate:"omitempty,oneof=active invited disabled"`
	unArchive bool               `json:"-"` // Internal use only.
}

// ArchiveUserAccountRequest defines the information needed to remove an existing account
// for a user. This will archive (soft-delete) the existing database entry.
type ArchiveUserAccountRequest struct {
	UserID    string `validate:"required,uuid"`
	AccountID string `validate:"required,uuid"`
}

// DeleteUserAccountRequest defines the information needed to delete an existing account
// for a user. This will hard delete the existing database entry.
type DeleteUserAccountRequest struct {
	UserID    string `validate:"required,uuid"`
	AccountID string `validate:"required,uuid"`
}

// UserAccountFindRequest defines the possible options to search for users accounts.
// By default archived user accounts will be excluded from response.
type UserAccountFindRequest struct {
	Where            *string       `schema:"where"`
	Args             []interface{} `schema:"args"`
	Order            []string      `schema:"order"`
	Limit            *uint         `schema:"limit"`
	Offset           *uint         `schema:"offset"`
	IncludedArchived bool          `schema:"included-archived"`
}

// UserAccountStatus represents the status of a user for an account.
type UserAccountStatus string

// UserAccountStatus values define the status field of a user account.
const (
	// UserAccountStatus_Active defines the state when a user can access an account.
	UserAccountStatus_Active UserAccountStatus = "active"
	// UserAccountStatus_Invited defined the state when a user has been invited to an
	// account.
	UserAccountStatus_Invited UserAccountStatus = "invited"
	// UserAccountStatus_Disabled defines the state when a user has been disabled from
	// accessing an account.
	UserAccountStatus_Disabled UserAccountStatus = "disabled"
)

// UserAccountStatus_Values provides list of valid UserAccountStatus values.
var UserAccountStatus_Values = []UserAccountStatus{
	UserAccountStatus_Active,
	UserAccountStatus_Invited,
	UserAccountStatus_Disabled,
}

// Scan supports reading the UserAccountStatus value from the database.
func (s *UserAccountStatus) Scan(value interface{}) error {
	asBytes, ok := value.([]byte)
	if !ok {
		return errors.New("Scan source is not []byte")
	}
	*s = UserAccountStatus(string(asBytes))
	return nil
}

// Value converts the UserAccountStatus value to be stored in the database.
func (s UserAccountStatus) Value() (driver.Value, error) {
	v := validator.New()

	errs := v.Var(s, "required,oneof=active invited disabled")
	if errs != nil {
		return nil, errs
	}

	return string(s), nil
}

// String converts the UserAccountStatus value to a string.
func (s UserAccountStatus) String() string {
	return string(s)
}

// UserAccountRole represents the role of a user for an account.
type UserAccountRole string

// UserAccountRole values define the role field of a user account.
const (
	// UserAccountRole_Admin defines the state of a user when they have admin
	// privileges for accessing an account. This role provides a user with full
	// access to an account.
	UserAccountRole_Admin UserAccountRole = auth.RoleAdmin
	// UserAccountRole_User defines the state of a user when they have basic
	// privileges for accessing an account. This role provies a user with the most
	// limited access to an account.
	UserAccountRole_User UserAccountRole = auth.RoleUser
)

// UserAccountRole_Values provides list of valid UserAccountRole values.
var UserAccountRole_Values = []UserAccountRole{
	UserAccountRole_Admin,
	UserAccountRole_User,
}

// String converts the UserAccountRole value to a string.
func (s UserAccountRole) String() string {
	return string(s)
}

// UserAccountRoles represents a set of roles for a user for an account.
type UserAccountRoles []UserAccountRole

// Scan supports reading the UserAccountRole value from the database.
func (s *UserAccountRoles) Scan(value interface{}) error {
	arr := &pq.StringArray{}
	if err := arr.Scan(value); err != nil {
		return err
	}

	for _, v := range *arr {
		*s = append(*s, UserAccountRole(v))
	}

	return nil
}

// Value converts the UserAccountRole value to be stored in the database.
func (s UserAccountRoles) Value() (driver.Value, error) {
	v := validator.New()

	var arr pq.StringArray
	for _, r := range s {
		errs := v.Var(r, "required,oneof=admin user")
		if errs != nil {
			return nil, errs
		}
		arr = append(arr, r.String())
	}

	return arr.Value()
}