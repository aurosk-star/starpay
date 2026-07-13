package paygateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL    string
	AppID      string
	AppSecret  string
	HTTPClient *http.Client
	Timeout    time.Duration
}

type Client struct {
	baseURL    string
	appID      string
	appSecret  string
	httpClient *http.Client
}

func NewClient(config Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("base url is required")
	}
	if strings.TrimSpace(config.AppID) == "" {
		return nil, errors.New("app id is required")
	}
	if strings.TrimSpace(config.AppSecret) == "" {
		return nil, errors.New("app secret is required")
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		timeout := config.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{
		baseURL:    baseURL,
		appID:      strings.TrimSpace(config.AppID),
		appSecret:  strings.TrimSpace(config.AppSecret),
		httpClient: httpClient,
	}, nil
}

func (c *Client) CreateOrder(ctx context.Context, input CreateOrderRequest) (*CreateOrderResult, error) {
	return requestJSON[CreateOrderResult](c, ctx, http.MethodPost, "/v1/open/orders", input)
}

func (c *Client) GetOrder(ctx context.Context, gatewayOrderNo string) (*Order, error) {
	var result struct {
		Order Order `json:"order"`
	}
	data, err := requestJSON[struct {
		Order Order `json:"order"`
	}](c, ctx, http.MethodGet, "/v1/open/orders/"+url.PathEscape(gatewayOrderNo), nil)
	if err != nil {
		return nil, err
	}
	result = *data
	return &result.Order, nil
}

func (c *Client) GetOrderByMerchant(ctx context.Context, merchantOrderNo string) (*Order, error) {
	data, err := requestJSON[struct {
		Order Order `json:"order"`
	}](c, ctx, http.MethodGet, "/v1/open/orders/by-merchant/"+url.PathEscape(merchantOrderNo), nil)
	if err != nil {
		return nil, err
	}
	return &data.Order, nil
}

func (c *Client) CloseOrder(ctx context.Context, gatewayOrderNo string) (*Order, error) {
	data, err := requestJSON[struct {
		Order Order `json:"order"`
	}](c, ctx, http.MethodPost, "/v1/open/orders/"+url.PathEscape(gatewayOrderNo)+"/close", nil)
	if err != nil {
		return nil, err
	}
	return &data.Order, nil
}

func (c *Client) CreateRefund(ctx context.Context, input CreateRefundRequest) (*CreateRefundResult, error) {
	return requestJSON[CreateRefundResult](c, ctx, http.MethodPost, "/v1/open/refunds", input)
}

func (c *Client) GetRefund(ctx context.Context, refundNo string) (*Refund, error) {
	data, err := requestJSON[struct {
		Refund Refund `json:"refund"`
	}](c, ctx, http.MethodGet, "/v1/open/refunds/"+url.PathEscape(refundNo), nil)
	if err != nil {
		return nil, err
	}
	return &data.Refund, nil
}

func (c *Client) GetRefundByMerchant(ctx context.Context, merchantRefundNo string) (*Refund, error) {
	data, err := requestJSON[struct {
		Refund Refund `json:"refund"`
	}](c, ctx, http.MethodGet, "/v1/open/refunds/by-merchant/"+url.PathEscape(merchantRefundNo), nil)
	if err != nil {
		return nil, err
	}
	return &data.Refund, nil
}

func (c *Client) signedURL(path string) (string, error) {
	parsed, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("app_id", c.appID)
	values.Set("request_id", newID("req"))
	values.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	values.Set("nonce", newID("nonce"))
	values.Set("sign", Sign(c.appSecret, values))
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func requestJSON[T any](client *Client, ctx context.Context, method string, path string, body any) (*T, error) {
	target, err := client.signedURL(path)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var envelope Response[T]
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, &APIError{
			HTTPStatus:   response.StatusCode,
			Code:         CodeInvalidResponse,
			Message:      "gateway returned an invalid API response",
			ResponseBody: responseBodySnippet(payload),
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || envelope.Error != nil || !strings.EqualFold(envelope.Code, CodeOK) {
		if envelope.Error != nil {
			envelope.Error.HTTPStatus = response.StatusCode
			return nil, envelope.Error
		}
		return nil, &APIError{
			HTTPStatus:   response.StatusCode,
			Code:         envelope.Code,
			Message:      envelope.Message,
			ResponseBody: responseBodySnippet(payload),
		}
	}
	return &envelope.Data, nil
}

func responseBodySnippet(payload []byte) string {
	const maxBytes = 4096
	if len(payload) > maxBytes {
		payload = payload[:maxBytes]
	}
	return strings.TrimSpace(string(payload))
}

func newID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return prefix + "_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:]))
}
