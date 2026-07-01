package handler

import (
	"errors"
	"io"
	"net/http"

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
	if err := ctx.Request.ParseForm(); err != nil {
		writeAlipayFail(ctx)
		return
	}
	rawBody, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		writeAlipayFail(ctx)
		return
	}
	channel := ctx.Query("channel")
	if channel == "" {
		channel = ctx.PostForm("channel")
	}
	result, err := h.payments.HandleNotify(ctx.Request.Context(), paymentsvc.NotifyInput{
		Channel: channel,
		Header:  ctx.Request.Header,
		Form:    ctx.Request.PostForm,
		RawBody: rawBody,
	})
	if err != nil || result.GatewayOrderNo == "" {
		writeAlipayFail(ctx)
		return
	}
	order, err := h.orders.FindOrderByGatewayOrderNo(ctx.Request.Context(), result.GatewayOrderNo)
	if err != nil {
		writeAlipayFail(ctx)
		return
	}
	if result.Status == "paid" {
		if order.Status == "paid" {
			writeAlipaySuccess(ctx)
			return
		}
		if _, err := h.orders.MarkPaid(ctx.Request.Context(), order.ID, result.ChannelTradeNo); err != nil {
			writeAlipayFail(ctx)
			return
		}
		writeAlipaySuccess(ctx)
		return
	}
	if result.Status == "closed" {
		if order.Status == "closed" {
			writeAlipaySuccess(ctx)
			return
		}
		if _, err := h.orders.CloseOrder(ctx.Request.Context(), order.ID); err != nil && !errors.Is(err, ordersvc.ErrOrderCannotBeClosed) {
			writeAlipayFail(ctx)
			return
		}
		writeAlipaySuccess(ctx)
		return
	}
	writeAlipaySuccess(ctx)
}

func writeAlipaySuccess(ctx *gin.Context) {
	ctx.String(http.StatusOK, "success")
}

func writeAlipayFail(ctx *gin.Context) {
	ctx.String(http.StatusOK, "fail")
}
