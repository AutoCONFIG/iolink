package domain

import "context"

type platformActorContextKey struct{}

type PlatformActor struct {
	ID           int64
	TokenVersion int
}

func WithPlatformActor(ctx context.Context, actor PlatformActor) context.Context {
	return context.WithValue(ctx, platformActorContextKey{}, actor)
}

func PlatformActorFromContext(ctx context.Context) (PlatformActor, bool) {
	actor, ok := ctx.Value(platformActorContextKey{}).(PlatformActor)
	return actor, ok
}
