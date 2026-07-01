package orderstest

import (
	"context"
	"testing"

	"payment-gateway/ent"
	appsvc "payment-gateway/internal/domain/apps/service"
)

func createEnabledApp(t *testing.T, client *ent.Client, appID string) {
	t.Helper()
	if _, err := appsvc.New(client).CreateApp(context.Background(), appsvc.ManageAppInput{
		AppID:  appID,
		Name:   appID,
		Status: "enabled",
	}); err != nil {
		t.Fatalf("CreateApp(%q) error = %v", appID, err)
	}
}
