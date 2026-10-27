//go:build !darwin

package disk

import "context"

func platform(context.Context, string, *Info) {}
