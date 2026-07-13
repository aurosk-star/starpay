package paygateway

import "time"

type Response[T any] struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Data    T         `json:"data"`
	Error   *APIError `json:"error"`
}

type APIError struct {
	HTTPStatus   int            `json:"-"`
	Code         string         `json:"code"`
	Message      string         `json:"message"`
	Details      map[string]any `json:"details,omitempty"`
	ResponseBody string         `json:"-"`
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
	ChannelAccountID   int                    `json:"channel_account_id,omitempty"`
	PayMethod          string                 `json:"pay_method,omitempty"`
	ProviderOrderNo    string                 `json:"provider_order_no,omitempty"`
	ChannelTradeNo     string                 `json:"channel_trade_no,omitempty"`
	ReturnURL          string                 `json:"return_url,omitempty"`
	Status             string                 `json:"status"`
	ExpiresAt          *time.Time             `json:"expires_at,omitempty"`
	PaidAt             *time.Time             `json:"paid_at,omitempty"`
	FailedAt           *time.Time             `json:"failed_at,omitempty"`
	FailureReason      string                 `json:"failure_reason,omitempty"`
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

type Refund struct {
	ID               int64          `json:"id,omitempty"`
	RefundNo         string         `json:"refund_no"`
	AppID            string         `json:"app_id,omitempty"`
	GatewayOrderNo   string         `json:"gateway_order_no"`
	MerchantOrderNo  string         `json:"merchant_order_no,omitempty"`
	MerchantRefundNo string         `json:"merchant_refund_no"`
	Channel          string         `json:"channel,omitempty"`
	ChannelAccountID int            `json:"channel_account_id,omitempty"`
	ChannelTradeNo   string         `json:"channel_trade_no,omitempty"`
	ChannelRefundNo  string         `json:"channel_refund_no,omitempty"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	Reason           string         `json:"reason,omitempty"`
	Status           string         `json:"status"`
	FailureReason    string         `json:"failure_reason,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	SucceededAt      *time.Time     `json:"succeeded_at,omitempty"`
	FailedAt         *time.Time     `json:"failed_at,omitempty"`
	CreatedAt        *time.Time     `json:"created_at,omitempty"`
	UpdatedAt        *time.Time     `json:"updated_at,omitempty"`
}

type CreateRefundRequest struct {
	GatewayOrderNo   string         `json:"gateway_order_no"`
	MerchantRefundNo string         `json:"merchant_refund_no"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	Reason           string         `json:"reason,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type CreateRefundResult struct {
	Created bool   `json:"created"`
	Refund  Refund `json:"refund"`
}
