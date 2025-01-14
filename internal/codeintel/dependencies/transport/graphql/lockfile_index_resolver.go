package graphql

import (
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies/shared"
)

type LockfileIndexResolver struct {
	lockfile shared.LockfileIndex
}

func NewExecutorResolver(executor shared.LockfileIndex) *LockfileIndexResolver {
	return &LockfileIndexResolver{lockfile: executor}
}

func (e *LockfileIndexResolver) ID() graphql.ID {
	return relay.MarshalID("LockfileIndex", (int64(e.lockfile.ID)))
}

func (e *LockfileIndexResolver) Lockfile() string {
	return e.lockfile.Lockfile
}
