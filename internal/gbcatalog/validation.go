package gbcatalog

import (
	"context"
	"time"
)

func boundedContext(parent context.Context, deadline time.Time, now time.Time) (context.Context, context.CancelFunc, error) {
	if parent == nil {
		parent = context.Background()
	}
	if deadline.IsZero() || !deadline.After(now) {
		return nil, func() {}, ErrDeadline
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	if err := ctx.Err(); err != nil {
		cancel()
		return nil, func() {}, err
	}
	return ctx, cancel, nil
}

func validPage(page Page) bool {
	if !validDeviceID(page.DeviceID) || page.SN <= 0 || page.SumNum < 0 || page.SumNum > MaxItems || len(page.Items) > MaxItems || len(page.Items) > page.SumNum {
		return false
	}
	seen := make(map[string]Item, len(page.Items))
	for _, item := range page.Items {
		if !validDeviceID(item.ChannelID) || len(item.Name) == 0 || len(item.Name) > MaxNameBytes {
			return false
		}
		if old, ok := seen[item.ChannelID]; ok && old != item {
			return false
		}
		seen[item.ChannelID] = item
	}
	if page.SumNum == 0 {
		return len(page.Items) == 0
	}
	return len(page.Items) > 0
}

func validDeviceID(value string) bool {
	if len(value) != 20 {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
