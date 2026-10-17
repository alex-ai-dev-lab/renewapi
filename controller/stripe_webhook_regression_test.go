package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"gorm.io/gorm"
	"strconv"
)

func TestStripeCheckoutUsesPersistedQuoteForPriceData(t *testing.T) {
	setupControllerAuditTestDB(t)
	configureStripeSubscriptionAuditTest(t)
	previousPrice := setting.StripePriceId
	setting.StripePriceId = "price_test"
	t.Cleanup(func() { setting.StripePriceId = previousPrice })
	user := createPaymentAuditUser(t)
	quote, err := stripeTopUpQuote(10, user.Group)
	require.NoError(t, err)
	order := model.TopUp{UserId: user.Id, TradeNo: "quote-transport", Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe, PaymentSnapshot: quote}
	require.NoError(t, order.Insert())
	calls := make(chan string, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/prices/price_test":
			_, _ = w.Write([]byte(`{"id":"price_test","object":"price","currency":"usd","unit_amount":999,"product":"prod_test"}`))
		case "/v1/checkout/sessions":
			require.NoError(t, r.ParseForm())
			require.Equal(t, strconv.FormatInt(quote.ExpectedMinor, 10), r.Form.Get("line_items[0][price_data][unit_amount]"))
			require.Equal(t, "usd", r.Form.Get("line_items[0][price_data][currency]"))
			require.Equal(t, "prod_test", r.Form.Get("line_items[0][price_data][product]"))
			require.Equal(t, "1", r.Form.Get("line_items[0][quantity]"))
			require.Equal(t, order.TradeNo, r.Header.Get("Idempotency-Key"))
			_, _ = w.Write([]byte(`{"id":"cs_quote","object":"checkout.session","url":"https://checkout.example.test/quote"}`))
		default:
			t.Errorf("unexpected Stripe path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	previousBackend := stripe.GetBackend(stripe.APIBackend)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		URL: stripe.String(upstream.URL), HTTPClient: upstream.Client(), MaxNetworkRetries: stripe.Int64(0),
	}))
	t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, previousBackend) })
	link, err := genStripeLink(order.TradeNo, "", "", 10, "", "")
	require.NoError(t, err)
	require.Equal(t, "https://checkout.example.test/quote", link)
	require.Equal(t, 2, len(calls))
	latest := model.GetTopUpByTradeNo(order.TradeNo)
	require.Equal(t, "cs_quote", latest.CheckoutSessionID)
}

func TestStripeSignedWebhookRetriesStorageFailureAndDeduplicates(t *testing.T) {
	db := setupControllerAuditTestDB(t)
	configureStripeSubscriptionAuditTest(t)
	previousPrice := setting.StripePriceId
	setting.StripePriceId = "price_test"
	t.Cleanup(func() { setting.StripePriceId = previousPrice })
	user := createPaymentAuditUser(t)
	order := model.TopUp{UserId: user.Id, TradeNo: "webhook", Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe,
		PaymentSnapshot: model.PaymentSnapshot{QuoteVersion: 1, Currency: "usd", ExpectedMinor: 1000, CreditQuota: 200, CheckoutSessionID: "cs_webhook", AllowPromotionCodes: true}}
	require.NoError(t, order.Insert())
	deliver := func(eventID, kind string) int {
		body, err := common.Marshal(map[string]any{"id": eventID, "object": "event", "type": kind,
			"data": map[string]any{"object": map[string]any{"id": "cs_webhook", "object": "checkout.session", "client_reference_id": "webhook", "status": "complete", "payment_status": "paid", "currency": "usd", "amount_subtotal": 1000, "amount_total": 800, "total_details": map[string]any{"amount_discount": 200, "amount_tax": 0}}}})
		require.NoError(t, err)
		timestamp := time.Now().Unix()
		signature := common.HmacSha256(fmt.Sprintf("%d.%s", timestamp, body), setting.StripeWebhookSecret)
		recorder := httptest.NewRecorder()
		router := gin.New()
		router.POST("/api/stripe/webhook", StripeWebhook)
		request := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(body))
		request.Header.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", timestamp, signature))
		router.ServeHTTP(recorder, request)
		return recorder.Code
	}
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:webhook-storage", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.AddError(errors.New("injected write failure"))
		}
	}))
	require.Equal(t, http.StatusServiceUnavailable, deliver("evt_first", "checkout.session.completed"))
	require.NoError(t, db.Callback().Update().Remove("test:webhook-storage"))
	require.Equal(t, http.StatusOK, deliver("evt_first", "checkout.session.completed"))
	require.Equal(t, http.StatusOK, deliver("evt_second", "checkout.session.async_payment_succeeded"))
	var latest model.User
	require.NoError(t, db.First(&latest, user.Id).Error)
	require.Equal(t, user.Quota+200, latest.Quota)
}

func TestStripeAdminReconciliationVerifiesOwnershipAndUsesSameReceipt(t *testing.T) {
	db := setupControllerAuditTestDB(t)
	previousLogs := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = previousLogs })
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	user := createPaymentAuditUser(t)
	order := model.TopUp{UserId: user.Id, TradeNo: "reconcile", Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe,
		PaymentSnapshot: model.PaymentSnapshot{QuoteVersion: 1, Currency: "usd", ExpectedMinor: 100, CreditQuota: 200}}
	require.NoError(t, order.Insert())
	checkout := &stripe.CheckoutSession{ID: "cs_reconcile", ClientReferenceID: "wrong_order", PaymentStatus: stripe.CheckoutSessionPaymentStatusPaid,
		Currency: stripe.CurrencyUSD, AmountSubtotal: 100, AmountTotal: 100}
	deliver := func() *httptest.ResponseRecorder {
		router := gin.New()
		router.POST("/reconcile", func(c *gin.Context) {
			c.Set("id", user.Id)
			adminReconcileStripeCheckout(c, func(string) (*stripe.CheckoutSession, error) { return checkout, nil })
		})
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/reconcile", bytes.NewBufferString(`{"trade_no":"reconcile","session_id":"cs_reconcile"}`)))
		return r
	}
	require.Contains(t, deliver().Body.String(), `"success":false`)
	require.NoError(t, db.First(&order, order.Id).Error)
	require.Empty(t, order.CheckoutSessionID)
	checkout.ClientReferenceID = order.TradeNo
	require.Contains(t, deliver().Body.String(), `"result":"fulfilled"`)
	require.Contains(t, deliver().Body.String(), `"result":"already_processed"`)
	var latest model.User
	require.NoError(t, db.First(&latest, user.Id).Error)
	require.Equal(t, user.Quota+200, latest.Quota)
}

func TestChannelTestCaptureRejectsOversizeWithoutRetainingIt(t *testing.T) {
	for _, asString := range []bool{false, true} {
		recorder := httptest.NewRecorder()
		writer := &limitedTestCapture{ResponseRecorder: recorder, limit: 8}
		var err error
		if asString {
			_, err = writer.WriteString("123456789")
		} else {
			_, err = writer.Write([]byte("123456789"))
		}
		require.ErrorIs(t, err, common.ErrUpstreamBodyTooLarge)
		require.True(t, writer.exceeded)
		require.Equal(t, "12345678", recorder.Body.String())
	}
}
