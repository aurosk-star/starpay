package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"payment-gateway/ent"
	"payment-gateway/internal/domain/apps/repository"
	webhookrepo "payment-gateway/internal/domain/webhooks/repository"
	platformauth "payment-gateway/internal/platform/auth"
)

const EventPaymentSucceeded = "payment.succeeded"
const EventPaymentFailed = "payment.failed"
const EventOrderExpired = "order.expired"
const ResourcePaymentOrder = "payment_order"
const ResourceRefund = "refund"
const maxDeliveryAttempts = 3
const maxResponseBodyBytes = 4096
const (
	webhookEventIDHeader    = "X-Pay-Gateway-Event-Id"
	webhookTimestampHeader  = "X-Pay-Gateway-Timestamp"
	webhookSignatureHeader  = "X-Pay-Gateway-Signature"
	webhookEventTypeHeader  = "X-Pay-Gateway-Event-Type"
	webhookDeliveryNoHeader = "X-Pay-Gateway-Delivery-No"
)

var ErrOrderRequired = errors.New("order is required")
var ErrEventTypeRequired = errors.New("event_type is required")
var ErrAppIDRequired = errors.New("app_id is required")
var ErrResourceTypeRequired = errors.New("resource_type is required")
var ErrResourceIDRequired = errors.New("resource_id is required")

type Service struct {
	apps                repository.Repository
	webhooks            webhookrepo.Repository
	now                 func() time.Time
	http                *http.Client
	enqueuer            Enqueuer
	secretEncryptionKey string
}

type Option func(*Service)

func WithEnqueuer(enqueuer Enqueuer) Option {
	return func(s *Service) {
		s.enqueuer = enqueuer
	}
}

func WithRedis(client *redis.Client) Option {
	return func(s *Service) {
		s.enqueuer = newRedisEnqueuer(client)
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(s *Service) {
		if client != nil {
			s.http = client
		}
	}
}

func WithSecretEncryptionKey(key string) Option {
	return func(s *Service) {
		s.secretEncryptionKey = key
	}
}

func New(client *ent.Client, opts ...Option) Service {
	svc := Service{
		apps:                repository.New(client),
		webhooks:            webhookrepo.New(client),
		now:                 time.Now,
		http:                &http.Client{Timeout: 10 * time.Second},
		secretEncryptionKey: "0123456789abcdef0123456789abcdef",
	}
	for _, opt := range opts {
		opt(&svc)
	}
	return svc
}

func (s Service) IsZero() bool {
	return s.webhooks.IsZero()
}

func (s Service) RecordPaymentSucceeded(ctx context.Context, order *ent.PaymentOrder) (*ent.WebhookEvent, error) {
	return s.recordOrderEvent(ctx, order, EventPaymentSucceeded, paymentSucceededPayload)
}

func (s Service) RecordPaymentFailed(ctx context.Context, order *ent.PaymentOrder) (*ent.WebhookEvent, error) {
	return s.recordOrderEvent(ctx, order, EventPaymentFailed, paymentFailedPayload)
}

func (s Service) RecordOrderExpired(ctx context.Context, order *ent.PaymentOrder) (*ent.WebhookEvent, error) {
	return s.recordOrderEvent(ctx, order, EventOrderExpired, orderExpiredPayload)
}

func (s Service) recordOrderEvent(ctx context.Context, order *ent.PaymentOrder, eventType string, payload func(*ent.PaymentOrder) map[string]any) (*ent.WebhookEvent, error) {
	if order == nil {
		return nil, ErrOrderRequired
	}
	return s.RecordResourceEvent(ctx, ResourceEventInput{
		EventType:      eventType,
		AppID:          order.AppID,
		ResourceType:   ResourcePaymentOrder,
		ResourceID:     order.GatewayOrderNo,
		GatewayOrderNo: order.GatewayOrderNo,
		PaymentOrderID: order.ID,
		Payload:        payload(order),
	})
}

type ResourceEventInput struct {
	EventType      string
	AppID          string
	ResourceType   string
	ResourceID     string
	GatewayOrderNo string
	RefundNo       string
	PaymentOrderID int
	Payload        map[string]any
}

func (s Service) RecordResourceEvent(ctx context.Context, input ResourceEventInput) (*ent.WebhookEvent, error) {
	input.EventType = strings.TrimSpace(input.EventType)
	input.AppID = strings.TrimSpace(input.AppID)
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	input.GatewayOrderNo = strings.TrimSpace(input.GatewayOrderNo)
	input.RefundNo = strings.TrimSpace(input.RefundNo)
	if input.EventType == "" {
		return nil, ErrEventTypeRequired
	}
	if input.AppID == "" {
		return nil, ErrAppIDRequired
	}
	if input.ResourceType == "" {
		return nil, ErrResourceTypeRequired
	}
	if input.ResourceID == "" {
		return nil, ErrResourceIDRequired
	}
	if input.Payload == nil {
		input.Payload = map[string]any{}
	}
	if existing, err := s.webhooks.FindEventByResource(ctx, input.EventType, input.ResourceType, input.ResourceID); err == nil {
		if _, deliveryErr := s.ensureDelivery(ctx, existing); deliveryErr != nil {
			return nil, deliveryErr
		}
		return existing, nil
	}
	if _, err := s.apps.FindByAppID(ctx, input.AppID); err != nil {
		return nil, err
	}
	eventID, err := newPublicID("evt", s.now())
	if err != nil {
		return nil, err
	}
	event, err := s.webhooks.CreateEvent(ctx, webhookrepo.CreateEventInput{
		EventID:        eventID,
		EventType:      input.EventType,
		AppID:          input.AppID,
		ResourceType:   input.ResourceType,
		ResourceID:     input.ResourceID,
		GatewayOrderNo: input.GatewayOrderNo,
		RefundNo:       input.RefundNo,
		PaymentOrderID: input.PaymentOrderID,
		Payload:        input.Payload,
	})
	if err != nil {
		if existing, findErr := s.webhooks.FindEventByResource(ctx, input.EventType, input.ResourceType, input.ResourceID); findErr == nil {
			event = existing
		} else {
			return nil, err
		}
	}
	delivery, err := s.ensureDelivery(ctx, event)
	if err != nil {
		return nil, err
	}
	if delivery != nil {
		if err := s.enqueueDelivery(ctx, delivery.ID); err != nil {
			return nil, err
		}
	}
	return event, nil
}

type ListDeliveriesInput struct {
	AppID          string
	EventType      string
	Status         string
	ResourceType   string
	ResourceID     string
	GatewayOrderNo string
	RefundNo       string
	Page           int
	PageSize       int
}

type ListDeliveriesResult struct {
	Items    []*ent.WebhookDelivery `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

func (s Service) ListDeliveries(ctx context.Context, input ListDeliveriesInput) (*ListDeliveriesResult, error) {
	items, total, err := s.webhooks.ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{
		AppID:          strings.TrimSpace(input.AppID),
		EventType:      strings.TrimSpace(input.EventType),
		Status:         strings.ToLower(strings.TrimSpace(input.Status)),
		ResourceType:   strings.TrimSpace(input.ResourceType),
		ResourceID:     strings.TrimSpace(input.ResourceID),
		GatewayOrderNo: strings.TrimSpace(input.GatewayOrderNo),
		RefundNo:       strings.TrimSpace(input.RefundNo),
		Page:           input.Page,
		PageSize:       input.PageSize,
	})
	if err != nil {
		return nil, err
	}
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return &ListDeliveriesResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s Service) RetryDelivery(ctx context.Context, id int) (*ent.WebhookDelivery, error) {
	delivery, err := s.webhooks.ResetDeliveryForRetry(ctx, id, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.enqueueDelivery(ctx, delivery.ID); err != nil {
		return nil, err
	}
	return delivery, nil
}

func (s Service) GetDelivery(ctx context.Context, id int) (*ent.WebhookDelivery, error) {
	return s.webhooks.FindDeliveryByID(ctx, id)
}

func (s Service) DeliverWebhook(ctx context.Context, id int) (*ent.WebhookDelivery, error) {
	delivery, err := s.webhooks.FindDeliveryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if delivery.Status == "succeeded" {
		return delivery, nil
	}
	event, err := s.webhooks.FindEventByID(ctx, delivery.EventID)
	if err != nil {
		return nil, err
	}
	attemptedAt := s.now()
	attemptCount := delivery.AttemptCount + 1
	statusCode, responseBody, deliveryErr := s.postDelivery(ctx, delivery, event)
	status := "pending"
	lastError := ""
	var nextAttemptAt *time.Time
	var succeededAt *time.Time
	if deliveryErr == nil && statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		status = "succeeded"
		succeededAt = &attemptedAt
	} else {
		if deliveryErr != nil {
			lastError = deliveryErr.Error()
		} else {
			lastError = http.StatusText(statusCode)
			if lastError == "" {
				lastError = "non-success status"
			}
		}
		if attemptCount >= maxDeliveryAttempts {
			status = "failed"
		} else {
			next := attemptedAt.Add(retryDelay(attemptCount))
			nextAttemptAt = &next
		}
	}
	return s.webhooks.UpdateDeliveryAttempt(ctx, delivery.ID, webhookrepo.DeliveryAttemptInput{
		Status:           status,
		AttemptCount:     attemptCount,
		NextAttemptAt:    nextAttemptAt,
		LastAttemptAt:    attemptedAt,
		LastStatusCode:   statusCode,
		LastResponseBody: responseBody,
		LastError:        lastError,
		SucceededAt:      succeededAt,
	})
}

func (s Service) ScanDueDeliveries(ctx context.Context, limit int) (int, error) {
	deliveries, err := s.webhooks.ListDueDeliveries(ctx, s.now(), limit)
	if err != nil {
		return 0, err
	}
	enqueued := 0
	for _, delivery := range deliveries {
		if err := s.enqueueDelivery(ctx, delivery.ID); err != nil {
			return enqueued, err
		}
		enqueued++
	}
	return enqueued, nil
}

func (s Service) ensureDelivery(ctx context.Context, event *ent.WebhookEvent) (*ent.WebhookDelivery, error) {
	if delivery, err := s.webhooks.FindDeliveryByEventID(ctx, event.ID); err == nil {
		return delivery, nil
	} else if !ent.IsNotFound(err) {
		return nil, err
	}
	app, err := s.apps.FindByAppID(ctx, event.AppID)
	if err != nil {
		return nil, err
	}
	targetURL := strings.TrimSpace(app.NotifyURL)
	if targetURL == "" {
		return nil, nil
	}
	deliveryNo, err := newPublicID("whd", s.now())
	if err != nil {
		return nil, err
	}
	return s.webhooks.CreateDelivery(ctx, webhookrepo.CreateDeliveryInput{
		DeliveryNo:     deliveryNo,
		EventID:        event.ID,
		AppID:          event.AppID,
		EventType:      event.EventType,
		ResourceType:   event.ResourceType,
		ResourceID:     event.ResourceID,
		GatewayOrderNo: event.GatewayOrderNo,
		RefundNo:       event.RefundNo,
		TargetURL:      targetURL,
		Status:         "pending",
		NextAttemptAt:  s.now(),
	})
}

func (s Service) enqueueDelivery(ctx context.Context, deliveryID int) error {
	if s.enqueuer == nil {
		return nil
	}
	return s.enqueuer.EnqueueWebhookDelivery(ctx, deliveryID)
}

func (s Service) postDelivery(ctx context.Context, delivery *ent.WebhookDelivery, event *ent.WebhookEvent) (int, string, error) {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return 0, "", err
	}
	timestamp := s.now().Unix()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.TargetURL, strings.NewReader(string(payload)))
	if err != nil {
		return 0, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(webhookEventIDHeader, event.EventID)
	request.Header.Set(webhookTimestampHeader, strconv.FormatInt(timestamp, 10))
	request.Header.Set(webhookEventTypeHeader, event.EventType)
	request.Header.Set(webhookDeliveryNoHeader, delivery.DeliveryNo)
	if signature, err := s.webhookSignature(ctx, event.AppID, timestamp, payload); err == nil && signature != "" {
		request.Header.Set(webhookSignatureHeader, signature)
	}
	client := s.http
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBodyBytes))
	if readErr != nil {
		return response.StatusCode, "", readErr
	}
	return response.StatusCode, string(body), nil
}

func retryDelay(attemptCount int) time.Duration {
	switch attemptCount {
	case 1:
		return 10 * time.Second
	case 2:
		return 30 * time.Second
	case 3:
		return 2 * time.Minute
	case 4:
		return 10 * time.Minute
	case 5:
		return 30 * time.Minute
	default:
		return 2 * time.Hour
	}
}

func (s Service) webhookSignature(ctx context.Context, appID string, timestamp int64, body []byte) (string, error) {
	app, err := s.apps.FindByAppID(ctx, appID)
	if err != nil {
		return "", err
	}
	if app.AppSecretCiphertext == "" {
		return "", errors.New("app secret is required")
	}
	secret, err := platformauth.DecryptSecret(s.secretEncryptionKey, app.AppSecretCiphertext)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func paymentSucceededPayload(order *ent.PaymentOrder) map[string]any {
	payload := map[string]any{
		"event_type":        EventPaymentSucceeded,
		"app_id":            order.AppID,
		"gateway_order_no":  order.GatewayOrderNo,
		"merchant_order_no": order.MerchantOrderNo,
		"amount":            order.Amount,
		"currency":          order.Currency,
		"channel":           order.Channel,
		"channel_trade_no":  order.ChannelTradeNo,
		"metadata":          order.Metadata,
	}
	if order.PaidAt != nil {
		payload["paid_at"] = order.PaidAt.Format(time.RFC3339)
	}
	return payload
}

func paymentFailedPayload(order *ent.PaymentOrder) map[string]any {
	payload := map[string]any{
		"event_type":        EventPaymentFailed,
		"app_id":            order.AppID,
		"gateway_order_no":  order.GatewayOrderNo,
		"merchant_order_no": order.MerchantOrderNo,
		"amount":            order.Amount,
		"currency":          order.Currency,
		"channel":           order.Channel,
		"pay_method":        order.PayMethod,
		"failure_reason":    order.FailureReason,
		"metadata":          order.Metadata,
	}
	if order.FailedAt != nil {
		payload["failed_at"] = order.FailedAt.Format(time.RFC3339)
	}
	return payload
}

func orderExpiredPayload(order *ent.PaymentOrder) map[string]any {
	payload := map[string]any{
		"event_type":        EventOrderExpired,
		"app_id":            order.AppID,
		"gateway_order_no":  order.GatewayOrderNo,
		"merchant_order_no": order.MerchantOrderNo,
		"amount":            order.Amount,
		"currency":          order.Currency,
		"status":            order.Status,
		"channel":           order.Channel,
		"pay_method":        order.PayMethod,
		"metadata":          order.Metadata,
	}
	if order.ExpiresAt != nil {
		payload["expires_at"] = order.ExpiresAt.Format(time.RFC3339)
	}
	if order.ClosedAt != nil {
		payload["closed_at"] = order.ClosedAt.Format(time.RFC3339)
	}
	return payload
}

func newPublicID(prefix string, now time.Time) (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:]))
	return prefix + "_" + now.UTC().Format("20060102") + "_" + encoded, nil
}
