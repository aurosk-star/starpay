package appstest

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
)

func TestDefaultReturnURLLifecycle(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:default_return_url?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := appsvc.New(client)
	created, err := svc.CreateApp(ctx, appsvc.ManageAppInput{
		AppID:            "return-url-app",
		Name:             "Return URL App",
		NotifyURL:        "https://merchant.example.com/webhook",
		DefaultReturnURL: "https://merchant.example.com/pay/result",
		Status:           "enabled",
	})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	if created.App.DefaultReturnURL != "https://merchant.example.com/pay/result" {
		t.Fatalf("DefaultReturnURL = %q, want created value", created.App.DefaultReturnURL)
	}

	updated, err := svc.UpdateApp(ctx, created.App.ID, appsvc.ManageAppInput{
		Name:             "Return URL App",
		NotifyURL:        "https://merchant.example.com/webhook",
		DefaultReturnURL: "https://merchant.example.com/pay/return",
		Status:           "enabled",
	})
	if err != nil {
		t.Fatalf("UpdateApp() error = %v", err)
	}
	if updated.DefaultReturnURL != "https://merchant.example.com/pay/return" {
		t.Fatalf("DefaultReturnURL = %q, want updated value", updated.DefaultReturnURL)
	}

	cleared, err := svc.UpdateApp(ctx, created.App.ID, appsvc.ManageAppInput{
		Name:             "Return URL App",
		NotifyURL:        "https://merchant.example.com/webhook",
		DefaultReturnURL: "",
		Status:           "enabled",
	})
	if err != nil {
		t.Fatalf("UpdateApp() clear error = %v", err)
	}
	if cleared.DefaultReturnURL != "" {
		t.Fatalf("DefaultReturnURL = %q, want cleared", cleared.DefaultReturnURL)
	}
}
