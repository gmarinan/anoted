//go:build !windows

package setup

import "context"

func verifyWindowsTitles(ctx context.Context) error {
	_ = ctx
	return nil
}
