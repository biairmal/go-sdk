package authz

import "context"

// noop allows everything.
type noop struct{}

// NewNoOp returns an Authorizer that allows every check. For tests and local
// development only.
func NewNoOp() Authorizer { return noop{} }

// Can always reports true.
func (noop) Can(context.Context, string) bool { return true }

// Require always returns nil.
func (noop) Require(context.Context, string) error { return nil }

// Permissions returns ["*"].
func (noop) Permissions(context.Context) []string { return []string{"*"} }

// RequireIn always returns nil.
func (noop) RequireIn(context.Context, Scope, string) error { return nil }
