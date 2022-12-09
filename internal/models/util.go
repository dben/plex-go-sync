package models

import (
	"context"
)

func IsDone(ctx *context.Context) bool {
	select {
	case <-(*ctx).Done():
		return true
	default:
		return false
	}
}

func GetConfig(ctx *context.Context) *Config {
	return (*ctx).Value("config").(*Config)
}
