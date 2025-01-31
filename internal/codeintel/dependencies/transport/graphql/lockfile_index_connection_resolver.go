package graphql

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
)

type LockfileIndexConnectionResolver struct {
	resolvers  []*LockfileIndexResolver
	totalCount int
	nextOffset *int
}

func NewLockfileIndexConnectionConnection(resolvers []*LockfileIndexResolver, totalCount int, nextOffset *int) *LockfileIndexConnectionResolver {
	return &LockfileIndexConnectionResolver{
		resolvers:  resolvers,
		totalCount: totalCount,
		nextOffset: nextOffset,
	}
}

func (r *LockfileIndexConnectionResolver) Nodes(ctx context.Context) (resolvers []graphqlbackend.LockfileIndexResolver) {
	resolvers = make([]graphqlbackend.LockfileIndexResolver, len(r.resolvers))
	for i, r := range r.resolvers {
		resolvers[i] = r
	}
	return resolvers
}

func (r *LockfileIndexConnectionResolver) TotalCount(ctx context.Context) int32 {
	return int32(r.totalCount)
}

func (r *LockfileIndexConnectionResolver) PageInfo(ctx context.Context) *graphqlutil.PageInfo {
	return graphqlutil.EncodeIntCursor(toInt32(r.nextOffset))
}

// toInt32 translates the given int pointer into an int32 pointer.
func toInt32(val *int) *int32 {
	if val == nil {
		return nil
	}

	v := int32(*val)
	return &v
}
