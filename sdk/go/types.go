package paygateway

import "time"

type Response[T any] struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Data    T         `json:"data"`
	Error   *APIError `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

type Order struct {
	ID                 int64                  `json:"id,omitempty"`
	GatewayOrderNo     string                 `json:"gateway_order_no"`
	AppID              string                 `json:"app_id,omitempty"`
	MerchantOrderNo    string                 `json:"merchant_order_no"`
	BusinessType       string                 `json:"business_type,omitempty"`
	Subject            string                 `json:"subject"`
	Description        string                 `json:"description,omitempty"`
	Amount             int64                  `json:"amount"`
	Currency           string                 `json:"currency"`
	SettlementAmount   int64                  `json:"settlement_amount,omitempty"`
	SettlementCurrency string                 `json:"settlement_currency,omitempty"`
	Channel            string                 `json:"channel,omitempty"`
	PayMethod          string                 `json:"pay_method,omitempty"`
	ChannelTradeNo     string                 `json:"channel_trade_no,omitempty"`
	ReturnURL          string                 `json:"return_url,omitempty"`
	Status             string                 `json:"status"`
	ExpiresAt          *time.Time             `json:"expires_at,omitempty"`
	PaidAt             *time.Time             `json:"paid_at,omitempty"`
	ClosedAt           *time.Time             `json:"closed_at,omitempty"`
	Metadata           map[string]any         `json:"metadata,omitempty"`
	CreatedAt          *time.Time             `json:"created_at,omitempty"`
	UpdatedAt          *time.Time             `json:"updated_at,omitempty"`
	Raw                map[string]interface{} `json:"-"`
}

type CreateOrderRequest struct {
	MerchantOrderNo  string         `json:"merchant_order_no"`
	BusinessType     string         `json:"business_type,omitempty"`
	Subject          string         `json:"subject"`
	Description      string         `json:"description,omitempty"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	Channel          string         `json:"channel,omitempty"`
	PayMethod        string         `json:"pay_method,omitempty"`
	PreferredChannel string         `json:"preferred_channel,omitempty"`
	ClientIP         string         `json:"client_ip,omitempty"`
	ReturnURL        string         `json:"return_url,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type CreateOrderResult struct {
	Created bool  `json:"created"`
	Order   Order `json:"order"`
	Payment struct {
		Status string `json:"status"`
		PayURL string `json:"pay_url,omitempty"`
	} `json:"payment"`
}
