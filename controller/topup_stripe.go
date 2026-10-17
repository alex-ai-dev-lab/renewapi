package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/price"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/thanhpk/randstr"
)

var stripeAdaptor = &StripeAdaptor{}

// StripePayRequest represents a payment request for Stripe checkout.
type StripePayRequest struct {
	// Amount is the quantity of units to purchase.
	Amount           int64  `json:"amount"`
	QuoteFingerprint string `json:"quote_fingerprint,omitempty"`
	// PaymentMethod specifies the payment method (e.g., "stripe").
	PaymentMethod string `json:"payment_method"`
	// SuccessURL is the optional custom URL to redirect after successful payment.
	// If empty, defaults to the server's console log page.
	SuccessURL string `json:"success_url,omitempty"`
	// CancelURL is the optional custom URL to redirect when payment is canceled.
	// If empty, defaults to the server's console topup page.
	CancelURL string `json:"cancel_url,omitempty"`
}

type StripeAdaptor struct {
}

func (*StripeAdaptor) RequestAmount(c *gin.Context, req *StripePayRequest) {
	if req.Amount < getStripeMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup())})
		return
	}
	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	quote, err := stripeTopUpQuote(req.Amount, group)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(float64(quote.ExpectedMinor)/100, 'f', 2, 64), "quote_fingerprint": stripeQuoteFingerprint(quote)})
}

func (*StripeAdaptor) RequestPay(c *gin.Context, req *StripePayRequest) {
	requestStripePayWithCheckout(c, req, genStripeLink)
}

func requestStripePayWithCheckout(
	c *gin.Context,
	req *StripePayRequest,
	createCheckout func(
		referenceId string,
		customerId string,
		email string,
		amount int64,
		successURL string,
		cancelURL string,
	) (string, error),
) {
	if req.PaymentMethod != model.PaymentMethodStripe {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}
	if req.Amount < getStripeMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup()), "data": 10})
		return
	}

	if req.SuccessURL != "" && common.ValidateRedirectURL(req.SuccessURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付成功重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	if req.CancelURL != "" && common.ValidateRedirectURL(req.CancelURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付取消重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil || user == nil {
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 获取充值用户失败 user_id=%d error=%q", id, err.Error()))
		}
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户信息失败"})
		return
	}
	quote, err := stripeTopUpQuote(req.Amount, user.Group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if req.QuoteFingerprint != "" && req.QuoteFingerprint != stripeQuoteFingerprint(quote) {
		c.JSON(http.StatusConflict, gin.H{"message": "Price changed. Refresh the quote before paying.", "success": false})
		return
	}
	chargedMoney := float64(quote.ExpectedMinor) / 100

	reference := fmt.Sprintf("new-api-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "ref_" + common.Sha1([]byte(reference))

	topUp := &model.TopUp{
		PaymentSnapshot: quote,
		UserId:          id,
		Amount:          req.Amount,
		Money:           chargedMoney,
		TradeNo:         referenceId,
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, referenceId, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	payLink, err := createCheckout(referenceId, user.StripeCustomer, user.Email, req.Amount, req.SuccessURL, req.CancelURL)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 创建 Checkout Session 失败 user_id=%d trade_no=%s amount=%d error=%q", id, referenceId, req.Amount, err.Error()))
		// An IO error does not prove Stripe did not create the Session.
		if updateErr := model.DB.Model(topUp).Where("status = ? AND payment_state = ?", common.TopUpStatusPending, "unpaid").Updates(map[string]any{"payment_state": "checkout_unknown", "fulfillment_error": "Checkout creation result unknown; reconcile before retrying"}).Error; updateErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 标记充值订单失败状态失败 user_id=%d trade_no=%s amount=%d error=%q", id, referenceId, req.Amount, updateErr.Error()))
		}
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Stripe 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f", id, referenceId, req.Amount, chargedMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": payLink,
		},
	})
}

func RequestStripeAmount(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestAmount(c, &req)
}

func RequestStripePay(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestPay(c, &req)
}

func StripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !isStripeWebhookEnabled() {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.URL.EscapedPath(), c.ClientIP()))
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	payload, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Stripe webhook 读取请求体失败 path=%q client_ip=%s error=%q", c.Request.URL.EscapedPath(), c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 收到请求 path=%q client_ip=%s payload_bytes=%d signature_present=%t", c.Request.URL.EscapedPath(), c.ClientIP(), len(payload), signature != ""))
	event, err := webhook.ConstructEventWithOptions(payload, signature, setting.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})

	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe webhook 验签失败 path=%q client_ip=%s error=%q", c.Request.URL.EscapedPath(), c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	callerIp := c.ClientIP()
	logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 验签成功 event_type=%s client_ip=%s path=%q", string(event.Type), callerIp, c.Request.URL.EscapedPath()))
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		err = sessionCompleted(ctx, event, callerIp)
	case stripe.EventTypeCheckoutSessionExpired:
		err = sessionExpired(ctx, event)
	case stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded:
		err = sessionAsyncPaymentSucceeded(ctx, event, callerIp)
	case stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
		err = sessionAsyncPaymentFailed(ctx, event, callerIp)
	default:
		logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 忽略事件 event_type=%s client_ip=%s", string(event.Type), callerIp))
	}

	if err != nil {
		logger.LogError(ctx, common.RedactErrorCredentials(fmt.Sprintf("Stripe webhook processing failed event_id=%s error=%v", event.ID, err), setting.StripeApiSecret, setting.StripeWebhookSecret))
		c.Status(http.StatusServiceUnavailable)
		return
	}
	c.Status(http.StatusOK)
}

func sessionCompleted(ctx context.Context, event stripe.Event, callerIp string) error {
	if event.GetObjectValue("status") != "complete" {
		return nil
	}
	paymentStatus := event.GetObjectValue("payment_status")
	if paymentStatus != "paid" && paymentStatus != "no_payment_required" {
		return nil
	}
	return fulfillOrder(ctx, event, event.GetObjectValue("client_reference_id"), event.GetObjectValue("customer"), callerIp)
}

func sessionAsyncPaymentSucceeded(ctx context.Context, event stripe.Event, callerIp string) error {
	if event.GetObjectValue("payment_status") != "paid" {
		return errors.New("async success without paid status")
	}
	return fulfillOrder(ctx, event, event.GetObjectValue("client_reference_id"), event.GetObjectValue("customer"), callerIp)
}

func sessionAsyncPaymentFailed(ctx context.Context, event stripe.Event, callerIp string) error {
	return model.EndStripeCheckout(event.GetObjectValue("client_reference_id"), event.GetObjectValue("id"), common.TopUpStatusFailed)
}

func fulfillOrder(ctx context.Context, event stripe.Event, referenceId string, customerId string, callerIp string) error {
	if referenceId == "" {
		return errors.New("Stripe paid event missing order reference")
	}
	var checkout stripe.CheckoutSession
	if err := common.Unmarshal(event.Data.Raw, &checkout); err != nil {
		return err
	}
	receipt := stripeReceiptFromCheckout(&checkout)
	result, err := model.CompleteStripeCheckout(receipt)
	if err != nil {
		return err
	}
	logger.LogInfo(ctx, fmt.Sprintf("Stripe payment processed trade_no=%s session_id=%s result=%s", referenceId, receipt.SessionID, result))
	return nil
}

func stripeReceiptFromCheckout(checkout *stripe.CheckoutSession) model.StripeReceipt {
	receipt := model.StripeReceipt{TradeNo: checkout.ClientReferenceID, SessionID: checkout.ID,
		Currency: string(checkout.Currency), Total: checkout.AmountTotal, Subtotal: checkout.AmountSubtotal}
	if checkout.Customer != nil {
		receipt.CustomerID = checkout.Customer.ID
	}
	if checkout.TotalDetails != nil {
		receipt.Discount, receipt.Tax, receipt.Shipping = checkout.TotalDetails.AmountDiscount, checkout.TotalDetails.AmountTax, checkout.TotalDetails.AmountShipping
	}
	return receipt
}

func sessionExpired(ctx context.Context, event stripe.Event) error {
	if event.GetObjectValue("status") != "expired" {
		return nil
	}
	return model.EndStripeCheckout(event.GetObjectValue("client_reference_id"), event.GetObjectValue("id"), common.TopUpStatusExpired)
}

// genStripeLink generates a Stripe Checkout session URL for payment.
// It creates a new checkout session with the specified parameters and returns the payment URL.
//
// Parameters:
//   - referenceId: unique reference identifier for the transaction
//   - customerId: existing Stripe customer ID (empty string if new customer)
//   - email: customer email address for new customer creation
//   - amount: quantity of units to purchase
//   - successURL: custom URL to redirect after successful payment (empty for default)
//   - cancelURL: custom URL to redirect when payment is canceled (empty for default)
//
// Returns the checkout session URL or an error if the session creation fails.
func genStripeLink(referenceId string, customerId string, email string, amount int64, successURL string, cancelURL string) (string, error) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return "", fmt.Errorf("无效的Stripe API密钥")
	}

	stripe.Key = setting.StripeApiSecret

	// Use custom URLs if provided, otherwise use defaults
	if successURL == "" {
		successURL = paymentReturnPath("/console/log")
	}
	if cancelURL == "" {
		cancelURL = paymentReturnPath("/console/topup")
	}

	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
	}
	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil || topUp.QuoteVersion == 0 {
		return "", errors.New("Stripe order snapshot missing")
	}
	configuredPrice, err := price.Get(topUp.CheckoutPriceID, nil)
	if err != nil {
		return "", err
	}
	if configuredPrice.Product == nil || !strings.EqualFold(string(configuredPrice.Currency), topUp.Currency) {
		return "", errors.New("Stripe price must use the quoted currency")
	}
	params.LineItems[0].PriceData = &stripe.CheckoutSessionLineItemPriceDataParams{
		Currency: stripe.String(topUp.Currency), Product: stripe.String(configuredPrice.Product.ID), UnitAmount: stripe.Int64(topUp.ExpectedMinor),
	}
	params.AllowPromotionCodes = stripe.Bool(topUp.AllowPromotionCodes)
	params.SetIdempotencyKey(referenceId)

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}

		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := session.New(params)
	if err != nil {
		return "", err
	}

	if err := model.BindStripeCheckout(referenceId, result.ID); err != nil {
		return "", err
	}
	return result.URL, nil
}

func GetChargedAmount(count float64, user model.User) float64 {
	topUpGroupRatio := common.GetTopupGroupRatio(user.Group)
	if topUpGroupRatio == 0 {
		topUpGroupRatio = 1
	}

	return count * topUpGroupRatio
}

func getStripePayMoney(amount float64, group string) float64 {
	originalAmount := amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = amount / common.QuotaPerUnit
	}
	// Using float64 for monetary calculations is acceptable here due to the small amounts involved
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	// apply optional preset discount by the original request amount (if configured), default 1.0
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(originalAmount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	payMoney := amount * setting.StripeUnitPrice * topupGroupRatio * discount
	return payMoney
}

func getStripeMinTopup() int64 {
	minTopup := setting.StripeMinTopUp
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		minTopup = minTopup * int(common.QuotaPerUnit)
	}
	return int64(minTopup)
}
