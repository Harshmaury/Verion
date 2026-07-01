package identity

import (
	"errors"
	"fmt"
)

// wrapRepoError passes known domain sentinel errors through unchanged
// so errors.Is works on callers, and wraps unknown errors to hide raw DB details.
func wrapRepoError(err error, op string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrTenantNotFound) ||
		errors.Is(err, ErrAlreadyExists) ||
		errors.Is(err, ErrHandleTaken) ||
		errors.Is(err, ErrTenantInactive) ||
		errors.Is(err, ErrIdentityTerminal) ||
		errors.Is(err, ErrIdentityInactive) ||
		errors.Is(err, ErrVersionConflict) ||
		errors.Is(err, ErrKeyNotFound) ||
		errors.Is(err, ErrKeyNotUsable) {
		return err
	}
	return fmt.Errorf("%s: %w", op, err)
}
