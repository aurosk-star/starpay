package paygateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	WebhookEventIDHeader    = "X-Pay-Gateway-Event-Id"
	WebhookTimestampHeader  = "X-Pay-Gateway-Timestamp"
	WebhookSignatureHeader  = "X-Pay-Gateway-Signature"
	WebhookEventTypeHeader  = "X-Pay-Gateway-Event-Type"
	WebhookDeliveryNoHeader = "X-Pay-Gateway-Delivery-No"
)

const DefaultWebhookTolerance = 5 * time.Minute

type WebhookVerificationOptions struct {
	Tolerance time.Duration
	Now       func() time.Time
}

type WebhookEvent struct {
	EventID          string         `json:"event_id,omitempty"`
	DeliveryNo       string         `json:"-"`
	Timestamp        int64          `json:"-"`
	EventType        string         `json:"event_type"`
	ResourceType     string         `json:"resource_type,omitempty"`
	ResourceID       string         `json:"resource_id,omitempty"`
	AppID            string         `json:"app_id"`
	GatewayOrderNo   string         `json:"gateway_order_no"`
	MerchantOrderNo  string         `json:"merchant_order_no"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	Status           string         `json:"status,omitempty"`
	Channel          string         `json:"channel,omitempty"`
	PayMethod        string         `json:"pay_method,omitempty"`
	ChannelTradeNo   string         `json:"channel_trade_no,omitempty"`
	RefundNo         string         `json:"refund_no,omitempty"`
	MerchantRefundNo string         `json:"merchant_refund_no,omitempty"`
	ChannelRefundNo  string         `json:"channel_refund_no,omitempty"`
	PaidAt           string         `json:"paid_at,omitempty"`
	FailedAt         string         `json:"failed_at,omitempty"`
	FailureReason    string         `json:"failure_reason,omitempty"`
	ExpiresAt        string         `json:"expires_at,omitempty"`
	ClosedAt         string         `json:"closed_at,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Raw              map[string]any `json:"-"`
}

func ParseWebhookRequest(request *http.Request, appSecret string) (*WebhookEvent, error) {
	return ParseWebhookRequestWithOptions(request, appSecret, WebhookVerificationOptions{})
}

func ParseWebhookRequestWithOptions(request *http.Request, appSecret string, options WebhookVerificationOptions) (*WebhookEvent, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	timestamp := request.Header.Get(WebhookTimestampHeader)
	if err := VerifyWebhookSignatureWithOptions(appSecret, timestamp, body, request.Header.Get(WebhookSignatureHeader), options); err != nil {
		return nil, err
	}
	event, err := ParseWebhookEvent(body)
	if err != nil {
		return nil, err
	}
	event.EventID = strings.TrimSpace(request.Header.Get(WebhookEventIDHeader))
	event.DeliveryNo = strings.TrimSpace(request.Header.Get(WebhookDeliveryNoHeader))
	event.Timestamp, _ = strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if headerEventType := strings.TrimSpace(request.Header.Get(WebhookEventTypeHeader)); headerEventType != "" && event.EventType == "" {
		event.EventType = headerEventType
	}
	return event, nil
}

func VerifyWebhookSignature(appSecret string, timestamp string, body []byte, signature string) error {
	return VerifyWebhookSignatureWithOptions(appSecret, timestamp, body, signature, WebhookVerificationOptions{})
}

func VerifyWebhookSignatureWithOptions(appSecret string, timestamp string, body []byte, signature string, options WebhookVerificationOptions) error {
	if strings.TrimSpace(appSecret) == "" {
		return errors.New("app secret is required")
	}
	if strings.TrimSpace(timestamp) == "" {
		return errors.New("webhook timestamp is required")
	}
	if strings.TrimSpace(signature) == "" {
		return errors.New("webhook signature is required")
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(signature))) {
		return errors.New("invalid webhook signature")
	}
	unixTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("invalid webhook timestamp")
	}
	tolerance := options.Tolerance
	if tolerance <= 0 {
		tolerance = DefaultWebhookTolerance
	}
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	signedAt := time.Unix(unixTimestamp, 0)
	current := now()
	if current.Sub(signedAt) > tolerance || signedAt.Sub(current) > tolerance {
		return errors.New("webhook timestamp is outside the allowed window")
	}
	return nil
}

func ParseWebhookEvent(body []byte) (*WebhookEvent, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	event.Raw = raw
	return &event, nil
}
