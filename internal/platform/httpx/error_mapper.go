package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
)

type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	Details    ErrorDetails
}

func WriteAPIError(ctx *gin.Context, err APIError) {
	JSONErrorWithDetails(ctx, err.HTTPStatus, err.Code, err.Message, err.Details)
}

func OrderAPIError(err error, fallbackCode string, fallbackMessage string) APIError {
	switch {
	case errors.Is(err, ordersvc.ErrMerchantOrderNoRequired):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeMissingRequiredField, Message: "merchant_order_no is required", Details: ErrorDetails{"field": "merchant_order_no"}}
	case errors.Is(err, ordersvc.ErrSubjectRequired):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeMissingRequiredField, Message: "subject is required", Details: ErrorDetails{"field": "subject"}}
	case errors.Is(err, ordersvc.ErrInvalidAmount):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeInvalidAmount, Message: "amount must be a positive integer in minor units", Details: ErrorDetails{"field": "amount"}}
	case errors.Is(err, ordersvc.ErrInvalidCurrency):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeInvalidCurrency, Message: "currency is invalid", Details: ErrorDetails{"field": "currency"}}
	case errors.Is(err, ordersvc.ErrUnsupportedCurrencyForChannel):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeCurrencyNotSupported, Message: "currency is not supported by channel", Details: ErrorDetails{}}
	case errors.Is(err, ordersvc.ErrIdempotencyConflict):
		return APIError{HTTPStatus: http.StatusConflict, Code: CodeIdempotencyConflict, Message: "merchant order already exists with different parameters", Details: ErrorDetails{}}
	case errors.Is(err, ordersvc.ErrOrderCannotBeClosed):
		return APIError{HTTPStatus: http.StatusConflict, Code: CodeOrderStatusNotAllowed, Message: "order status does not allow this operation", Details: ErrorDetails{}}
	case errors.Is(err, paymentsvc.ErrOrderExpired):
		return APIError{HTTPStatus: http.StatusConflict, Code: CodeOrderExpired, Message: "payment order is expired", Details: ErrorDetails{}}
	case errors.Is(err, paymentsvc.ErrOrderNotPayable):
		return APIError{HTTPStatus: http.StatusConflict, Code: CodeOrderStatusNotAllowed, Message: "order status does not allow this operation", Details: ErrorDetails{}}
	case errors.Is(err, ordersvc.ErrAppNotFound):
		return APIError{HTTPStatus: http.StatusUnauthorized, Code: CodeAppNotFound, Message: "app not found", Details: ErrorDetails{}}
	case errors.Is(err, ordersvc.ErrAppDisabled):
		return APIError{HTTPStatus: http.StatusForbidden, Code: CodeAppDisabled, Message: "app is disabled", Details: ErrorDetails{}}
	case errors.Is(err, paymentsvc.ErrProviderUnavailable):
		return APIError{HTTPStatus: http.StatusServiceUnavailable, Code: CodeChannelUnavailable, Message: "payment channel is unavailable", Details: ErrorDetails{"retryable": true}}
	case errors.Is(err, paymentsvc.ErrPayMethodRequired):
		return APIError{HTTPStatus: http.StatusBadRequest, Code: CodeMissingRequiredField, Message: "pay_method is required", Details: ErrorDetails{"field": "pay_method"}}
	default:
		return APIError{HTTPStatus: http.StatusInternalServerError, Code: fallbackCode, Message: fallbackMessage, Details: ErrorDetails{}}
	}
}
