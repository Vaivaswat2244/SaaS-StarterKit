package signup

import (
	"context"
	"time"

	"geeks-accelerator/oss/saas-starter-kit/internal/account"
	"geeks-accelerator/oss/saas-starter-kit/internal/platform/auth"
	"geeks-accelerator/oss/saas-starter-kit/internal/platform/web/webcontext"
	"geeks-accelerator/oss/saas-starter-kit/internal/user"
	"geeks-accelerator/oss/saas-starter-kit/internal/user_account"
	"github.com/jmoiron/sqlx"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/go-playground/validator.v9"
)

type ctxKeyTagUniqueName int

const KeyTagUniqueName ctxKeyTagUniqueName = 1


type ctxKeyTagUniqueEmail int

const KeyTagUniqueEmail ctxKeyTagUniqueEmail = 1



// validate holds the settings and caches for validating request struct values.
var validate *validator.Validate

// Validator returns the current init validator.
func Validator() *validator.Validate {
	if validate == nil {
		validate = webcontext.Validator()

		validate.RegisterValidationCtx("unique-name", func(ctx context.Context, fl validator.FieldLevel) bool {
			if fl.Field().String() == "invalid" {
				return false
			}

			cv := ctx.Value(KeyTagUniqueName)
			if cv == nil {
				return false
			}

			if v, ok := cv.(bool); ok {
				return v
			}

			return false
		})

		validate.RegisterValidationCtx("unique-email", func(ctx context.Context, fl validator.FieldLevel) bool {
			if fl.Field().String() == "invalid" {
				return false
			}

			cv := ctx.Value(KeyTagUniqueEmail)
			if cv == nil {
				return false
			}

			if v, ok := cv.(bool); ok {
				return v
			}

			return false
		})
	}
	return validate
}


// Signup performs the steps needed to create a new account, new user and then associate
// both records with a new user_account entry.
func Signup(ctx context.Context, claims auth.Claims, dbConn *sqlx.DB, req SignupRequest, now time.Time) (*SignupResult, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "internal.signup.Signup")
	defer span.Finish()

	// Validate the user email address is unique in the database.
	uniqEmail, err := user.UniqueEmail(ctx, dbConn, req.User.Email, "")
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, KeyTagUniqueEmail, uniqEmail)

	// Validate the account name is unique in the database.
	uniqName, err := account.UniqueName(ctx, dbConn, req.Account.Name, "")
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, KeyTagUniqueName, uniqName)

	// Validate the request.
	err = Validator().StructCtx(ctx, req)
	if err != nil {
		return nil, err
	}

	var resp SignupResult

	// UserCreateRequest contains information needed to create a new User.
	userReq := user.UserCreateRequest{
		FirstName:       req.User.FirstName,
		LastName:        req.User.LastName,
		Email:           req.User.Email,
		Password:        req.User.Password,
		PasswordConfirm: req.User.PasswordConfirm,
		Timezone:        req.Account.Timezone,
	}

	// Execute user creation.
	resp.User, err = user.Create(ctx, claims, dbConn, userReq, now)
	if err != nil {
		return nil, err
	}

	accountStatus := account.AccountStatus_Active
	accountReq := account.AccountCreateRequest{
		Name:          req.Account.Name,
		Address1:      req.Account.Address1,
		Address2:      req.Account.Address2,
		City:          req.Account.City,
		Region:        req.Account.Region,
		Country:       req.Account.Country,
		Zipcode:       req.Account.Zipcode,
		Status:        &accountStatus,
		Timezone:      req.Account.Timezone,
		SignupUserID:  &resp.User.ID,
		BillingUserID: &resp.User.ID,
	}

	// Execute account creation.
	resp.Account, err = account.Create(ctx, claims, dbConn, accountReq, now)
	if err != nil {
		return nil, err
	}

	// Associate the created user with the new account. The first user for the account will
	// always have the role of admin.
	ua := user_account.UserAccountCreateRequest{
		UserID:    resp.User.ID,
		AccountID: resp.Account.ID,
		Roles:     []user_account.UserAccountRole{user_account.UserAccountRole_Admin},
		//Status:  Use default value
	}

	_, err = user_account.Create(ctx, claims, dbConn, ua, now)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}
