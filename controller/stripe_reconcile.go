package controller

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
)

// AdminReconcileStripeCheckout recovers an uncertain Session creation using a
// provider-confirmed Session ID from the Stripe dashboard. No payment is created.
func AdminReconcileStripeCheckout(c *gin.Context) {
	adminReconcileStripeCheckout(c, func(id string) (*stripe.CheckoutSession, error) {
		stripe.Key = setting.StripeApiSecret
		return session.Get(id, &stripe.CheckoutSessionParams{Params: stripe.Params{Context: c.Request.Context()}})
	})
}

func adminReconcileStripeCheckout(c *gin.Context, fetch func(string) (*stripe.CheckoutSession, error)) {
	var req struct {
		TradeNo   string `json:"trade_no"`
		SessionID string `json:"session_id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.TradeNo == "" || req.SessionID == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	checkout, err := fetch(req.SessionID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Unable to verify Stripe Session"})
		return
	}
	if checkout == nil || checkout.ID != req.SessionID || checkout.ClientReferenceID != req.TradeNo {
		common.ApiErrorMsg(c, "Checkout Session does not belong to this order")
		return
	}
	if err := model.BindStripeCheckout(req.TradeNo, checkout.ID); err != nil {
		common.ApiError(c, err)
		return
	}
	result := "pending"
	if checkout.Status == stripe.CheckoutSessionStatusExpired {
		err = model.EndStripeCheckout(req.TradeNo, checkout.ID, common.TopUpStatusExpired)
		result = "expired"
	} else if checkout.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid || checkout.PaymentStatus == stripe.CheckoutSessionPaymentStatusNoPaymentRequired {
		var paymentResult model.PaymentResult
		paymentResult, err = model.CompleteStripeCheckout(stripeReceiptFromCheckout(checkout))
		result = string(paymentResult)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLog(c.GetInt("id"), model.LogTypeManage, fmt.Sprintf("Stripe reconciliation trade_no=%s session_id=%s result=%s", req.TradeNo, checkout.ID, result))
	common.ApiSuccess(c, gin.H{"result": result})
}
