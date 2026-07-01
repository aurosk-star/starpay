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
	channel string
	result  *paymentprovider.NotifyResult
	err     error
	called  bool
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
	_ = req
	p.called = true
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
