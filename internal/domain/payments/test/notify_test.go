package paymentstest

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	channelrepo "payment-gateway/internal/domain/channels/repository"
	paymentprovider "payment-gateway/internal/domain/payments/provider"
	paymentsvc "payment-gateway/internal/domain/payments/service"
)

type fakeNotifyProvider struct {
	channel   string
	result    *paymentprovider.NotifyResult
	err       error
	called    bool
	accountID int
}

func (p *fakeNotifyProvider) Channel() string {
	return p.channel
}

func (p *fakeNotifyProvider) StartPayment(ctx context.Context, req paymentprovider.StartPaymentRequest) (*paymentprovider.StartPaymentResult, error) {
	_ = ctx
	_ = req
	return nil, errors.New("not used")
}

func (p *fakeNotifyProvider) ParseNotify(ctx context.Context, req paymentprovider.NotifyRequest) (*paymentprovider.NotifyResult, error) {
	_ = ctx
	p.called = true
	if req.ChannelAccount != nil {
		p.accountID = req.ChannelAccount.ID
	}
	return p.result, p.err
}

func TestHandleNotifyUsesEnabledChannelProvider(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:payment_notify_provider?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	channels := channelrepo.New(client)
	if _, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{
		Channel: "alipay",
		Name:    "Alipay Sandbox",
		Enabled: true,
		Env:     "sandbox",
		Config:  map[string]any{"app_id": "app-1", "alipay_public_key": "public-key"},
	}); err != nil {
		t.Fatalf("Create channel account error = %v", err)
	}
	provider := &fakeNotifyProvider{
		channel: "alipay",
		result: &paymentprovider.NotifyResult{
			Channel:        "alipay",
			GatewayOrderNo: "GW202607010001",
			ChannelTradeNo: "2026070122000000001",
			Status:         "paid",
			Amount:         9900,
			Currency:       "CNY",
			Raw:            map[string]any{"trade_status": "TRADE_SUCCESS"},
		},
	}
	service := paymentsvc.New(
		paymentsvc.WithChannelRepository(channels),
		paymentsvc.WithProvider(provider),
	)

	result, err := service.HandleNotify(ctx, paymentsvc.NotifyInput{
		Channel: "alipay",
		Form: url.Values{
			"out_trade_no": {"GW202607010001"},
		},
	})
	if err != nil {
		t.Fatalf("HandleNotify() error = %v", err)
	}
	if !provider.called {
		t.Fatal("provider was not called")
	}
	if result.GatewayOrderNo != "GW202607010001" || result.ChannelTradeNo != "2026070122000000001" || result.Status != "paid" {
		t.Fatalf("result = %#v, want normalized paid notify", result)
	}
	if result.ChannelAccountID == 0 || result.ChannelAccountID != provider.accountID {
		t.Fatalf("ChannelAccountID = %d, provider account = %d", result.ChannelAccountID, provider.accountID)
	}
}

func TestHandleNotifyRequiresAccountIDWhenChannelHasMultipleAccounts(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:payment_notify_multiple_accounts?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	channels := channelrepo.New(client)
	first, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{Channel: "alipay", Name: "First", Enabled: true, Env: "prod", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Create first account error = %v", err)
	}
	if _, err := channels.Create(ctx, channelrepo.CreateChannelAccountInput{Channel: "alipay", Name: "Second", Enabled: true, Env: "prod", Config: map[string]any{}}); err != nil {
		t.Fatalf("Create second account error = %v", err)
	}
	provider := &fakeNotifyProvider{channel: "alipay", result: &paymentprovider.NotifyResult{Channel: "alipay", GatewayOrderNo: "pay_001", Status: "paid", Amount: 100, Currency: "CNY"}}
	service := paymentsvc.New(paymentsvc.WithChannelRepository(channels), paymentsvc.WithProvider(provider))

	if _, err := service.HandleNotify(ctx, paymentsvc.NotifyInput{Channel: "alipay"}); !errors.Is(err, paymentsvc.ErrChannelAccountRequired) {
		t.Fatalf("HandleNotify() error = %v, want ErrChannelAccountRequired", err)
	}
	result, err := service.HandleNotify(ctx, paymentsvc.NotifyInput{Channel: "alipay", ChannelAccountID: first.ID})
	if err != nil {
		t.Fatalf("HandleNotify(account) error = %v", err)
	}
	if result.ChannelAccountID != first.ID || provider.accountID != first.ID {
		t.Fatalf("result/provider account = %d/%d, want %d", result.ChannelAccountID, provider.accountID, first.ID)
	}
}

func TestHandleNotifyRejectsProviderWithoutNotifySupport(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:payment_notify_unsupported?mode=memory&cache=shared&_fk=1")
	defer client.Close()

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
	service := paymentsvc.New(
		paymentsvc.WithChannelRepository(channels),
		paymentsvc.WithProvider(&fakeProvider{channel: "alipay"}),
	)

	if _, err := service.HandleNotify(ctx, paymentsvc.NotifyInput{Channel: "alipay"}); !errors.Is(err, paymentsvc.ErrNotifyUnsupported) {
		t.Fatalf("HandleNotify() error = %v, want ErrNotifyUnsupported", err)
	}
}
