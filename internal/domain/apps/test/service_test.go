package appstest

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	appsvc "payment-gateway/internal/domain/apps/service"
	"payment-gateway/internal/platform/auth"
	platformauth "payment-gateway/internal/platform/auth"
)

const testEncryptionKey = "0123456789abcdef0123456789abcdef"

func TestCreateAppHashesSecretAndReturnsPlaintextOnce(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_app?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := appsvc.New(client, appsvc.WithSecretEncryptionKey(testEncryptionKey))
	result, err := svc.CreateApp(ctx, appsvc.ManageAppInput{
		AppID:            "snsgo",
		Name:             "snsgo",
		NotifyURL:        "https://snsgo.example.com/payment/webhook",
		DefaultReturnURL: "https://snsgo.example.com/payment/result",
		AllowedIPs:       []string{"10.0.0.1"},
		Status:           "enabled",
	})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	if result.AppSecret == "" {
		t.Fatal("AppSecret is empty")
	}
	if result.App.AppSecretHash == "" || result.App.AppSecretHash == result.AppSecret {
		t.Fatalf("secret hash = %q, plaintext = %q", result.App.AppSecretHash, result.AppSecret)
	}
	if !platformauth.CheckPassword(result.App.AppSecretHash, result.AppSecret) {
		t.Fatal("stored hash does not match returned secret")
	}
	if result.App.AppSecretCiphertext == "" {
		t.Fatal("AppSecretCiphertext is empty")
	}
	if result.App.AppSecretCiphertext == result.AppSecret {
		t.Fatal("AppSecretCiphertext stored plaintext secret")
	}
	if result.App.NotifyURL != "https://snsgo.example.com/payment/webhook" {
		t.Fatalf("NotifyURL = %q, want merchant webhook", result.App.NotifyURL)
	}
	if result.App.DefaultReturnURL != "https://snsgo.example.com/payment/result" {
		t.Fatalf("DefaultReturnURL = %q, want browser return URL", result.App.DefaultReturnURL)
	}
	secret, err := auth.DecryptSecret(testEncryptionKey, result.App.AppSecretCiphertext)
	if err != nil {
		t.Fatalf("DecryptSecret() error = %v", err)
	}
	if secret != result.AppSecret {
		t.Fatal("decrypted signing secret does not match returned secret")
	}
}

func TestUpdateAppChangesMetadataWithoutChangingSecret(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:update_app?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := appsvc.New(client, appsvc.WithSecretEncryptionKey(testEncryptionKey))
	created, err := svc.CreateApp(ctx, appsvc.ManageAppInput{AppID: "billing", Name: "Billing", Status: "enabled"})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	originalHash := created.App.AppSecretHash
	originalCiphertext := created.App.AppSecretCiphertext

	updated, err := svc.UpdateApp(ctx, created.App.ID, appsvc.ManageAppInput{
		Name:             "Billing API",
		NotifyURL:        "https://billing.example.com/webhook",
		DefaultReturnURL: "https://billing.example.com/payments/result",
		AllowedIPs:       []string{"192.168.1.10"},
		Status:           "disabled",
	})
	if err != nil {
		t.Fatalf("UpdateApp() error = %v", err)
	}
	if updated.Name != "Billing API" || updated.Status != "disabled" {
		t.Fatalf("updated app = %#v", updated)
	}
	if updated.AppSecretHash != originalHash {
		t.Fatal("UpdateApp changed app secret hash")
	}
	if updated.AppSecretCiphertext != originalCiphertext {
		t.Fatal("UpdateApp changed app secret ciphertext")
	}
	if updated.NotifyURL != "https://billing.example.com/webhook" {
		t.Fatalf("NotifyURL = %q, want merchant webhook", updated.NotifyURL)
	}
	if updated.DefaultReturnURL != "https://billing.example.com/payments/result" {
		t.Fatalf("DefaultReturnURL = %q, want browser return URL", updated.DefaultReturnURL)
	}

	cleared, err := svc.UpdateApp(ctx, created.App.ID, appsvc.ManageAppInput{
		Name:             "Billing API",
		NotifyURL:        "https://billing.example.com/webhook",
		DefaultReturnURL: "",
		AllowedIPs:       []string{"192.168.1.10"},
		Status:           "enabled",
	})
	if err != nil {
		t.Fatalf("UpdateApp() clear default return url error = %v", err)
	}
	if cleared.DefaultReturnURL != "" {
		t.Fatalf("DefaultReturnURL = %q, want cleared", cleared.DefaultReturnURL)
	}
}

func TestResetSecretRotatesSecretHash(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:reset_secret?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := appsvc.New(client, appsvc.WithSecretEncryptionKey(testEncryptionKey))
	created, err := svc.CreateApp(ctx, appsvc.ManageAppInput{AppID: "ops", Name: "Ops", Status: "enabled"})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	originalHash := created.App.AppSecretHash
	originalCiphertext := created.App.AppSecretCiphertext

	reset, err := svc.ResetSecret(ctx, created.App.ID)
	if err != nil {
		t.Fatalf("ResetSecret() error = %v", err)
	}
	if reset.AppSecret == "" {
		t.Fatal("reset secret is empty")
	}
	if reset.App.AppSecretHash == originalHash {
		t.Fatal("secret hash did not change")
	}
	if reset.App.AppSecretCiphertext == "" || reset.App.AppSecretCiphertext == originalCiphertext {
		t.Fatal("secret ciphertext did not rotate")
	}
	if !platformauth.CheckPassword(reset.App.AppSecretHash, reset.AppSecret) {
		t.Fatal("stored reset hash does not match returned secret")
	}
	secret, err := auth.DecryptSecret(testEncryptionKey, reset.App.AppSecretCiphertext)
	if err != nil {
		t.Fatalf("DecryptSecret() error = %v", err)
	}
	if secret != reset.AppSecret {
		t.Fatal("decrypted reset secret does not match returned secret")
	}
}
