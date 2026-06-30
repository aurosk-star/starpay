package channelstest

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	channelsvc "payment-gateway/internal/domain/channels/service"
)

func TestCreateWechatChannelMasksSecrets(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:create_wechat_channel?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := channelsvc.New(client)
	view, err := svc.CreateChannelAccount(ctx, channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信支付生产商户号",
		Env:     "prod",
		Config: map[string]any{
			"app_id":      "wx_app_id",
			"mch_id":      "mch_123",
			"api_v3_key":  "secret-v3",
			"serial_no":   "serial-1",
			"private_key": "pem-private-key",
		},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount() error = %v", err)
	}
	if view.Config["api_v3_key"] != "********" {
		t.Fatalf("api_v3_key = %#v, want masked", view.Config["api_v3_key"])
	}
	if view.Config["private_key"] != "********" {
		t.Fatalf("private_key = %#v, want masked", view.Config["private_key"])
	}
}

func TestUpdateAlipayChannelMasksSecrets(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:update_alipay_channel?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := channelsvc.New(client)
	created, err := svc.CreateChannelAccount(ctx, channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝生产商户号",
		Env:     "prod",
		Config: map[string]any{
			"app_id":              "app-1",
			"private_key":         "private-key",
			"alipay_public_key":    "public-key",
			"alipay_root_cert_sn":  "root-sn",
			"alipay_public_key_sn": "public-sn",
		},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount() error = %v", err)
	}

	view, err := svc.UpdateChannelAccount(ctx, created.ID, channelsvc.ManageChannelAccountInput{
		Channel: "alipay",
		Name:    "支付宝生产商户号-2",
		Env:     "prod",
		Config: map[string]any{
			"app_id":            "app-2",
			"private_key":       "private-key-2",
			"alipay_public_key": "public-key-2",
		},
	})
	if err != nil {
		t.Fatalf("UpdateChannelAccount() error = %v", err)
	}
	if view.Name != "支付宝生产商户号-2" {
		t.Fatalf("Name = %q, want updated", view.Name)
	}
	if view.Config["private_key"] != "********" {
		t.Fatalf("private_key = %#v, want masked", view.Config["private_key"])
	}
	if view.Config["alipay_public_key"] != "********" {
		t.Fatalf("alipay_public_key = %#v, want masked", view.Config["alipay_public_key"])
	}
}

func TestUpdateChannelKeepsExistingSensitiveConfigWhenBlank(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:update_keep_sensitive_channel?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := channelsvc.New(client)
	created, err := svc.CreateChannelAccount(ctx, channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信支付",
		Env:     "prod",
		Config: map[string]any{
			"app_id":      "wx_app",
			"api_v3_key":  "old-secret",
			"private_key": "old-private-key",
		},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount() error = %v", err)
	}

	view, err := svc.UpdateChannelAccount(ctx, created.ID, channelsvc.ManageChannelAccountInput{
		Channel: "wechat",
		Name:    "微信支付更新",
		Env:     "prod",
		Config: map[string]any{
			"app_id":      "wx_app_updated",
			"api_v3_key":  "",
			"private_key": "",
		},
	})
	if err != nil {
		t.Fatalf("UpdateChannelAccount() error = %v", err)
	}
	if view.Config["api_v3_key"] != "********" || view.Config["private_key"] != "********" {
		t.Fatalf("view config = %#v, want masked sensitive fields", view.Config)
	}

	found, err := client.ChannelAccount.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get channel account: %v", err)
	}
	if found.Config["api_v3_key"] != "old-secret" {
		t.Fatalf("stored api_v3_key = %#v, want old secret", found.Config["api_v3_key"])
	}
	if found.Config["private_key"] != "old-private-key" {
		t.Fatalf("stored private_key = %#v, want old private key", found.Config["private_key"])
	}
	if found.Config["app_id"] != "wx_app_updated" {
		t.Fatalf("stored app_id = %#v, want updated app id", found.Config["app_id"])
	}
}

func TestDisablePaypalChannel(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:disable_paypal_channel?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := channelsvc.New(client)
	created, err := svc.CreateChannelAccount(ctx, channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal Sandbox",
		Env:     "sandbox",
		Config: map[string]any{
			"client_id":     "client-1",
			"client_secret": "secret-1",
		},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount() error = %v", err)
	}

	view, err := svc.DisableChannelAccount(ctx, created.ID)
	if err != nil {
		t.Fatalf("DisableChannelAccount() error = %v", err)
	}
	if view.Enabled {
		t.Fatal("Enabled = true, want false")
	}
}

func TestFindChannelAccountReturnsMaskedDetail(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:find_channel_detail?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := channelsvc.New(client)
	created, err := svc.CreateChannelAccount(ctx, channelsvc.ManageChannelAccountInput{
		Channel: "paypal",
		Name:    "PayPal Production",
		Env:     "prod",
		Config: map[string]any{
			"client_id":     "client-1",
			"client_secret": "secret-1",
		},
	})
	if err != nil {
		t.Fatalf("CreateChannelAccount() error = %v", err)
	}

	found, err := svc.FindChannelAccount(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindChannelAccount() error = %v", err)
	}
	if found.ID != created.ID || found.Name != "PayPal Production" {
		t.Fatalf("found = %#v, want created channel", found)
	}
	if found.Config["client_secret"] != "********" {
		t.Fatalf("client_secret = %#v, want masked", found.Config["client_secret"])
	}
}

func TestRejectsInvalidChannel(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:reject_invalid_channel?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	svc := channelsvc.New(client)
	_, err := svc.CreateChannelAccount(ctx, channelsvc.ManageChannelAccountInput{
		Channel: "stripe",
		Name:    "Invalid",
		Env:     "prod",
		Config:  map[string]any{},
	})
	if err == nil {
		t.Fatal("CreateChannelAccount() error = nil, want invalid channel error")
	}
}
