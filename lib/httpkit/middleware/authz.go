package middleware

import (
	"errors"
	"net/http"

	"github.com/biairmal/go-sdk/lib/authz"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
)

// RequirePermission returns a middleware that lets a request through only when
// a grants perm. Place it after Auth so the claims are in the context. A denial
// is written as the standard error envelope with the status of the error a
// returns: 401 without claims, 403 without the permission. A nil a fails
// closed with 500.
func RequirePermission(a authz.Authorizer, perm string) func(http.Handler) http.Handler {
	return RequireAny(a, perm)
}

// RequireAny is RequirePermission for a route that any one of perms unlocks.
// With no perms every request is denied with 403.
func RequireAny(a authz.Authorizer, perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if a == nil {
				handler.WriteErrorResponse(w, http.StatusInternalServerError,
					errorz.Internal().WithMessage("authz: no authorizer configured"))
				return
			}
			ctx := authz.WithMemo(r.Context())
			var err error = errorz.Forbidden().WithMessage("authz: no permission given for route")
			for _, perm := range perms {
				perr := a.Require(ctx, perm)
				if perr == nil {
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				err = perr
				if !errors.Is(perr, errorz.ErrForbidden) {
					break // 401 or a source failure: another perm cannot fix it
				}
			}
			handler.WriteErrorResponse(w, handler.StatusCodeFromError(err), err)
		})
	}
}
