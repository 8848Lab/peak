package cli

import (
	"errors"
	"fmt"

	"github.com/8848lab/peak/internal/api"
)

// authError returns a friendly "not logged in" error when err represents an
// auth failure returned by the Himalaya API (e.g. a revoked/expired token),
// or nil when err is not an auth error — callers should fall through to
// their own generic wrap in that case.
func authError(err error) error {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.IsAuthError() {
		return fmt.Errorf("not logged in — run `peak login` first")
	}
	return nil
}
