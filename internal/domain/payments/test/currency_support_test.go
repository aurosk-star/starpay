package paymentstest

import (
	"testing"

	paymentsvc "payment-gateway/internal/domain/payments/service"
)

func TestChannelSupportsCurrency(t *testing.T) {
	cases := []struct {
		channel  string
		currency string
		want     bool
	}{
		{channel: "alipay", currency: "CNY", want: true},
		{channel: "alipay", currency: "USD", want: false},
		{channel: "wechat", currency: "CNY", want: true},
		{channel: "wechat", currency: "USD", want: false},
		{channel: "paypal", currency: "USD", want: true},
		{channel: "paypal", currency: "JPY", want: true},
		{channel: "unknown", currency: "CNY", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.channel+"_"+tc.currency, func(t *testing.T) {
			if got := paymentsvc.ChannelSupportsCurrency(tc.channel, tc.currency); got != tc.want {
				t.Fatalf("ChannelSupportsCurrency(%q, %q) = %v, want %v", tc.channel, tc.currency, got, tc.want)
			}
		})
	}
}
