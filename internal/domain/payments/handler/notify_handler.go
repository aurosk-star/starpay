package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	ordersvc "payment-gateway/internal/domain/orders/service"
	paymentsvc "payment-gateway/internal/domain/payments/service"
)

type NotifyHandler struct {
	payments paymentsvc.Service
	orders   ordersvc.Service
}

func NewNotify(paymentService paymentsvc.Service, orderService ordersvc.Service) NotifyHandler {
	return NotifyHandler{payments: paymentService, orders: orderService}
}

func (h NotifyHandler) Handle(ctx *gin.Context) {
	channel := notifyChannel(ctx)
	channelAccountID, err := notifyChannelAccountID(ctx)
	if err != nil {
		writeNotifyFail(ctx, channel)
		return
	}
	rawBody, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		writeNotifyFail(ctx, channel)
		return
	}
	if !isJSONNotify(ctx) {
		ctx.Request.Body = io.NopCloser(bytes.NewReader(rawBody))
		if err := ctx.Request.ParseForm(); err != nil {
			writeNotifyFail(ctx, channel)
			return
		}
		if channel == "" {
			channel = ctx.PostForm("channel")
		}
	}
	result, err := h.payments.HandleNotify(ctx.Request.Context(), paymentsvc.NotifyInput{
		Channel:          channel,
		ChannelAccountID: channelAccountID,
		Header:           ctx.Request.Header,
		Form:             ctx.Request.PostForm,
		RawBody:          rawBody,
	})
	if err != nil || result.GatewayOrderNo == "" {
		writeNotifyFail(ctx, channel)
		return
	}
	order, err := h.orders.FindOrderByGatewayOrderNo(ctx.Request.Context(), result.GatewayOrderNo)
	if err != nil {
		writeNotifyFail(ctx, channel)
		return
	}
	if _, err := h.orders.ApplyPaymentResult(ctx.Request.Context(), order.ID, ordersvc.PaymentResultInput{
		Channel:          result.Channel,
		ChannelAccountID: result.ChannelAccountID,
		ChannelTradeNo:   result.ChannelTradeNo,
		Status:           result.Status,
		Amount:           result.Amount,
		Currency:         result.Currency,
		FailureReason:    result.FailureReason,
	}); err != nil {
		writeNotifyFail(ctx, result.Channel)
		return
	}
	writeNotifySuccess(ctx, result.Channel)
}

func notifyChannelAccountID(ctx *gin.Context) (int, error) {
	value := strings.TrimSpace(ctx.Query("channel_account_id"))
	if value == "" {
		return 0, nil
	}
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid channel_account_id")
	}
	return id, nil
}

func notifyChannel(ctx *gin.Context) string {
	channel := strings.TrimSpace(ctx.Query("channel"))
	if channel != "" {
		return channel
	}
	if isWechatNotify(ctx) {
		return "wechat"
	}
	return ""
}

func isWechatNotify(ctx *gin.Context) bool {
	return strings.TrimSpace(ctx.GetHeader("Wechatpay-Signature")) != "" &&
		strings.TrimSpace(ctx.GetHeader("Wechatpay-Timestamp")) != "" &&
		strings.TrimSpace(ctx.GetHeader("Wechatpay-Nonce")) != "" &&
		strings.TrimSpace(ctx.GetHeader("Wechatpay-Serial")) != ""
}

func isJSONNotify(ctx *gin.Context) bool {
	return strings.Contains(strings.ToLower(ctx.GetHeader("Content-Type")), "application/json")
}

func writeNotifySuccess(ctx *gin.Context, channel string) {
	if strings.EqualFold(strings.TrimSpace(channel), "wechat") {
		writeWechatSuccess(ctx)
		return
	}
	writeAlipaySuccess(ctx)
}

func writeNotifyFail(ctx *gin.Context, channel string) {
	if strings.EqualFold(strings.TrimSpace(channel), "wechat") {
		writeWechatFail(ctx)
		return
	}
	writeAlipayFail(ctx)
}

func writeAlipaySuccess(ctx *gin.Context) {
	ctx.String(http.StatusOK, "success")
}

func writeAlipayFail(ctx *gin.Context) {
	ctx.String(http.StatusOK, "fail")
}

func writeWechatSuccess(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

func writeWechatFail(ctx *gin.Context) {
	ctx.Status(http.StatusInternalServerError)
	_ = json.NewEncoder(ctx.Writer).Encode(gin.H{"code": "FAIL", "message": "失败"})
}
