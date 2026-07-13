package paymentstest

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent"
	"payment-gateway/ent/enttest"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentprovider "payment-gateway/internal/domain/payments/provider"
	paymentsvc "payment-gateway/internal/domain/payments/service"
)

type fakeProvider struct {
	channel string
	called  bool
	req     paymentprovider.StartPaymentRequest
}

func (p *fakeProvider) Channel() string {
	return p.channel
}

func (p *fakeProvider) StartPayment(ctx context.Context, req paymentprovider.StartPaymentRequest) (*paymentprovider.StartPaymentResult, error) {
	_ = ctx
	p.called = true
	p.req = req
	return &paymentprovider.StartPaymentResult{
		Status:          "pending",
		Channel:         req.Channel,
		PayMethod:       req.PayMethod,
		ProviderOrderNo: "provider_" + req.Order.GatewayOrderNo,
		PayURL:          "https://pay.example.com/" + req.Order.GatewayOrderNo,
	}, nil
}

func TestStartPaymentUsesEnabledAlipayProvider(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:payment_provider_enabled?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	order := createPaymentOrder(t, client)
	channels := channelrepo.New(client)
	if _, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{
		Channel: "alipay",
		Name:    "Alipay Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1"},
	}); err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	provider := &fakeProvider{channel: "alipay"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channels),
		paymentsvc.WithProvider(provider),
	)

	result, err := paymentService.StartPayment(ctx, paymentsvc.StartPaymentInput{
		Order:     order,
		PayMethod: "alipay",
		Channel:   "alipay",
		ReturnURL: "https://merchant.example.com/return",
		NotifyURL: "https://pay.example.com/v1/channel/notify",
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if !provider.called {
		t.Fatal("provider was not called")
	}
	if provider.req.ChannelAccount == nil || provider.req.ChannelAccount.ID == 0 {
		t.Fatalf("ChannelAccount = %#v, want enabled account", provider.req.ChannelAccount)
	}
	if provider.req.ReturnURL != "https://merchant.example.com/return" {
		t.Fatalf("ReturnURL = %q, want request return URL", provider.req.ReturnURL)
	}
	notifyURL, err := url.Parse(provider.req.NotifyURL)
	if err != nil {
		t.Fatalf("parse NotifyURL error = %v", err)
	}
	if notifyURL.Scheme+"://"+notifyURL.Host+notifyURL.Path != "https://pay.example.com/v1/channel/notify" ||
		notifyURL.Query().Get("channel") != "alipay" ||
		notifyURL.Query().Get("channel_account_id") != strconv.Itoa(provider.req.ChannelAccount.ID) {
		t.Fatalf("NotifyURL = %q, want channel and account binding", provider.req.NotifyURL)
	}
	if result.ProviderOrderNo != "provider_"+order.GatewayOrderNo {
		t.Fatalf("ProviderOrderNo = %q, want provider result", result.ProviderOrderNo)
	}
	if result.ChannelAccountID != provider.req.ChannelAccount.ID {
		t.Fatalf("ChannelAccountID = %d, want %d", result.ChannelAccountID, provider.req.ChannelAccount.ID)
	}
}

func TestStartPaymentUsesSelectedChannelAccountID(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:payment_provider_selected_account?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	order := createPaymentOrder(t, client)
	channels := channelrepo.New(client)
	first, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{
		Channel: "alipay",
		Name:    "Alipay First",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "first"},
	})
	if err != nil {
		t.Fatalf("Create first channel account error = %v", err)
	}
	second, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{
		Channel: "alipay",
		Name:    "Alipay Second",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "second"},
	})
	if err != nil {
		t.Fatalf("Create second channel account error = %v", err)
	}
	provider := &fakeProvider{channel: "alipay"}
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channels),
		paymentsvc.WithProvider(provider),
	)

	_, err = paymentService.StartPayment(ctx, paymentsvc.StartPaymentInput{
		Order:            order,
		PayMethod:        "alipay",
		Channel:          "alipay",
		ChannelAccountID: first.ID,
	})
	if err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
	if provider.req.ChannelAccount.ID != first.ID {
		t.Fatalf("ChannelAccount.ID = %d, want first account %d", provider.req.ChannelAccount.ID, first.ID)
	}

	_, err = paymentService.StartPayment(ctx, paymentsvc.StartPaymentInput{
		Order:            order,
		PayMethod:        "alipay",
		Channel:          "alipay",
		ChannelAccountID: second.ID,
	})
	if err != nil {
		t.Fatalf("StartPayment() second error = %v", err)
	}
	if provider.req.ChannelAccount.ID != second.ID {
		t.Fatalf("ChannelAccount.ID = %d, want second account %d", provider.req.ChannelAccount.ID, second.ID)
	}
}

func TestStartPaymentRejectsUnavailableAlipayAccount(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:payment_provider_unavailable?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createEnabledApp(t, client, "snsgo")

	order := createPaymentOrder(t, client)
	channels := channelrepo.New(client)
	paymentService := paymentsvc.New(
		paymentsvc.WithChannelRepository(channels),
		paymentsvc.WithProvider(&fakeProvider{channel: "alipay"}),
	)

	_, err := paymentService.StartPayment(ctx, paymentsvc.StartPaymentInput{
		Order:     order,
		PayMethod: "alipay",
		Channel:   "alipay",
		ReturnURL: "https://merchant.example.com/return",
	})
	if !errors.Is(err, paymentsvc.ErrProviderUnavailable) {
		t.Fatalf("StartPayment() error = %v, want ErrProviderUnavailable", err)
	}
}

func createPaymentOrder(t *testing.T, client *ent.Client) *ent.PaymentOrder {
	t.Helper()
	orderService := ordersvc.New(client)
	order, err := orderService.CreateOrder(t.Context(), ordersvc.ManageOrderInput{
		AppID:           "snsgo",
		MerchantOrderNo: "biz_" + t.Name(),
		Subject:         "Pro 会员",
		Amount:          9900,
		Currency:        "CNY",
		PayMethod:       "alipay",
		Channel:         "alipay",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	return order
}
