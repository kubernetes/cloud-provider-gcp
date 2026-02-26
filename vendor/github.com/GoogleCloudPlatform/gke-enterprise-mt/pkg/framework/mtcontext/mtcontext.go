package mtcontext

import (
	"context"
	"fmt"
)

type tenantUIDKeyType struct{}

// tenantUIDKey is the key for the tenant UID in the context.
var tenantUIDKey tenantUIDKeyType

// TenantUIDFromContext returns the tenant UID from the context.
func TenantUIDFromContext(ctx context.Context) any {
	v := ctx.Value(tenantUIDKey)
	if v == nil {
		return nil
	}
	return fmt.Sprintf("tenant-uid:%v", v)
}

// ContextWithTenantUID returns a context with the tenant UID.
func ContextWithTenantUID(ctx context.Context, tenantUID string) context.Context {
	return context.WithValue(ctx, tenantUIDKey, tenantUID)
}
