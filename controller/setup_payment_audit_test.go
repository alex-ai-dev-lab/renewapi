package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupControllerAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousRedisEnabled := common.RedisEnabled
	previousSetup := constant.Setup
	previousSelfUse := operation_setting.SelfUseModeEnabled
	previousDemo := operation_setting.DemoSiteEnabled

	common.OptionMapRWMutex.Lock()
	optionMapWasNil := common.OptionMap == nil
	if optionMapWasNil {
		common.OptionMap = make(map[string]string)
	}
	previousSelfUseOption, hadSelfUseOption := common.OptionMap["SelfUseModeEnabled"]
	previousDemoOption, hadDemoOption := common.OptionMap["DemoSiteEnabled"]
	common.OptionMapRWMutex.Unlock()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Option{},
		&model.Setup{},
		&model.TopUp{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
	); err != nil {
		t.Fatalf("failed to migrate audit test tables: %v", err)
	}

	model.DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	constant.Setup = false
	operation_setting.SelfUseModeEnabled = false
	operation_setting.DemoSiteEnabled = false

	t.Cleanup(func() {
		model.DB = previousDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.RedisEnabled = previousRedisEnabled
		constant.Setup = previousSetup
		operation_setting.SelfUseModeEnabled = previousSelfUse
		operation_setting.DemoSiteEnabled = previousDemo

		common.OptionMapRWMutex.Lock()
		if optionMapWasNil {
			common.OptionMap = nil
		} else {
			if hadSelfUseOption {
				common.OptionMap["SelfUseModeEnabled"] = previousSelfUseOption
			} else {
				delete(common.OptionMap, "SelfUseModeEnabled")
			}
			if hadDemoOption {
				common.OptionMap["DemoSiteEnabled"] = previousDemoOption
			} else {
				delete(common.OptionMap, "DemoSiteEnabled")
			}
		}
		common.OptionMapRWMutex.Unlock()

		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func postSetupAuditRequest(t *testing.T, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	body, err := common.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal setup request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = request
	PostSetup(c)
	return response
}

func TestPostSetupRollsBackTransactionAndSanitizesError(t *testing.T) {
	db := setupControllerAuditTestDB(t)
	const sensitiveDatabaseError = "sensitive database detail must not reach client"
	if err := db.Callback().Create().Before("gorm:create").Register("test:fail_setup_root_create", func(tx *gorm.DB) {
		user, ok := tx.Statement.Dest.(*model.User)
		if ok && user.Role == common.RoleRootUser {
			tx.AddError(errors.New(sensitiveDatabaseError))
		}
	}); err != nil {
		t.Fatalf("failed to register setup failure callback: %v", err)
	}

	response := postSetupAuditRequest(t, map[string]any{
		"username":           "rootadmin",
		"password":           "password123",
		"confirmPassword":    "password123",
		"SelfUseModeEnabled": true,
		"DemoSiteEnabled":    true,
	})

	if !strings.Contains(response.Body.String(), "系统初始化失败") {
		t.Fatalf("expected sanitized setup failure, got %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), sensitiveDatabaseError) {
		t.Fatalf("response leaked database error: %s", response.Body.String())
	}

	var setupCount int64
	if err := db.Model(&model.Setup{}).Count(&setupCount).Error; err != nil {
		t.Fatalf("failed to count setup records: %v", err)
	}
	if setupCount != 0 {
		t.Fatalf("setup row count = %d, want 0 after rollback", setupCount)
	}
	var rootCount int64
	if err := db.Model(&model.User{}).Where("role = ?", common.RoleRootUser).Count(&rootCount).Error; err != nil {
		t.Fatalf("failed to count root users: %v", err)
	}
	if rootCount != 0 {
		t.Fatalf("root user count = %d, want 0 after rollback", rootCount)
	}
	if constant.Setup || operation_setting.SelfUseModeEnabled || operation_setting.DemoSiteEnabled {
		t.Fatal("runtime setup state changed after failed transaction")
	}
}

func TestPostSetupCreatesSingletonRootAndDurableOptions(t *testing.T) {
	db := setupControllerAuditTestDB(t)
	payload := map[string]any{
		"username":           "rootadmin",
		"password":           "password123",
		"confirmPassword":    "password123",
		"SelfUseModeEnabled": true,
		"DemoSiteEnabled":    false,
	}

	first := postSetupAuditRequest(t, payload)
	if !strings.Contains(first.Body.String(), `"success":true`) {
		t.Fatalf("expected first setup to succeed, got %s", first.Body.String())
	}
	second := postSetupAuditRequest(t, payload)
	if !strings.Contains(second.Body.String(), "系统已经初始化完成") {
		t.Fatalf("expected duplicate setup rejection, got %s", second.Body.String())
	}

	var setupCount int64
	if err := db.Model(&model.Setup{}).Count(&setupCount).Error; err != nil {
		t.Fatalf("failed to count setup records: %v", err)
	}
	if setupCount != 1 {
		t.Fatalf("setup row count = %d, want 1", setupCount)
	}
	var rootCount int64
	if err := db.Model(&model.User{}).Where("role = ?", common.RoleRootUser).Count(&rootCount).Error; err != nil {
		t.Fatalf("failed to count root users: %v", err)
	}
	if rootCount != 1 {
		t.Fatalf("root user count = %d, want 1", rootCount)
	}
	var selfUse model.Option
	if err := db.First(&selfUse, "key = ?", "SelfUseModeEnabled").Error; err != nil {
		t.Fatalf("failed to read self-use option: %v", err)
	}
	if selfUse.Value != "true" {
		t.Fatalf("SelfUseModeEnabled = %q, want true", selfUse.Value)
	}
	if !constant.Setup || !operation_setting.SelfUseModeEnabled || operation_setting.DemoSiteEnabled {
		t.Fatal("runtime setup state was not published after commit")
	}
}

func newPaymentAuditContext(userID int) (*gin.Context, *httptest.ResponseRecorder) {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/payment", nil)
	c.Set("id", userID)
	return c, response
}

func createPaymentAuditUser(t *testing.T) *model.User {
	user := &model.User{
		Username:    "payment-user",
		Password:    "password123",
		DisplayName: "Payment User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Email:       "payment@example.com",
		Group:       "default",
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create payment test user: %v", err)
	}
	return user
}

func configureStripeTopUpAuditTest(t *testing.T) {
	previousMinTopUp := setting.StripeMinTopUp
	setting.StripeMinTopUp = 1
	t.Cleanup(func() { setting.StripeMinTopUp = previousMinTopUp })
}

func configureStripeSubscriptionAuditTest(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	previousPaymentSetting := *paymentSetting
	previousStripeSecret := setting.StripeApiSecret
	previousStripeWebhookSecret := setting.StripeWebhookSecret

	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	setting.StripeApiSecret = "sk_test_controller_audit"
	setting.StripeWebhookSecret = "whsec_controller_audit"
	t.Cleanup(func() {
		*paymentSetting = previousPaymentSetting
		setting.StripeApiSecret = previousStripeSecret
		setting.StripeWebhookSecret = previousStripeWebhookSecret
	})
}

func createStripeSubscriptionAuditPlan(t *testing.T) *model.SubscriptionPlan {
	plan := &model.SubscriptionPlan{
		Title:         "Stripe audit plan",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		StripePriceId: "price_controller_audit",
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to create subscription plan: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
	return plan
}

func TestStripeTopUpPersistsPendingOrderBeforeCheckout(t *testing.T) {
	setupControllerAuditTestDB(t)
	configureStripeTopUpAuditTest(t)
	user := createPaymentAuditUser(t)
	c, response := newPaymentAuditContext(user.Id)
	checkoutCalled := false

	requestStripePayWithCheckout(c, &StripePayRequest{Amount: 100, PaymentMethod: model.PaymentMethodStripe}, func(referenceID, customerID, email string, amount int64, successURL, cancelURL string) (string, error) {
		checkoutCalled = true
		var topUp model.TopUp
		if err := model.DB.Where("trade_no = ?", referenceID).First(&topUp).Error; err != nil {
			t.Fatalf("pending topup did not exist before checkout: %v", err)
		}
		if topUp.Status != common.TopUpStatusPending || topUp.PaymentProvider != model.PaymentProviderStripe {
			t.Fatalf("topup before checkout = status %q provider %q", topUp.Status, topUp.PaymentProvider)
		}
		return "https://checkout.example.test/topup", nil
	})

	if !checkoutCalled || !strings.Contains(response.Body.String(), `"message":"success"`) {
		t.Fatalf("expected successful topup checkout, called=%v response=%s", checkoutCalled, response.Body.String())
	}
}

func TestStripeTopUpMarksOrderFailedWhenCheckoutFails(t *testing.T) {
	setupControllerAuditTestDB(t)
	configureStripeTopUpAuditTest(t)
	user := createPaymentAuditUser(t)
	c, response := newPaymentAuditContext(user.Id)
	const upstreamSecret = "stripe upstream secret failure detail"

	requestStripePayWithCheckout(c, &StripePayRequest{Amount: 100, PaymentMethod: model.PaymentMethodStripe}, func(referenceID, customerID, email string, amount int64, successURL, cancelURL string) (string, error) {
		return "", errors.New(upstreamSecret)
	})

	var topUp model.TopUp
	if err := model.DB.First(&topUp).Error; err != nil {
		t.Fatalf("failed to read topup after checkout failure: %v", err)
	}
	if topUp.Status != common.TopUpStatusFailed {
		t.Fatalf("topup status = %q, want %q", topUp.Status, common.TopUpStatusFailed)
	}
	if !strings.Contains(response.Body.String(), "拉起支付失败") || strings.Contains(response.Body.String(), upstreamSecret) {
		t.Fatalf("checkout error was not sanitized: %s", response.Body.String())
	}
}

func TestStripeSubscriptionPersistsPendingOrderBeforeCheckout(t *testing.T) {
	setupControllerAuditTestDB(t)
	configureStripeSubscriptionAuditTest(t)
	user := createPaymentAuditUser(t)
	plan := createStripeSubscriptionAuditPlan(t)
	body, err := common.Marshal(SubscriptionStripePayRequest{PlanId: plan.Id})
	if err != nil {
		t.Fatalf("failed to marshal subscription request: %v", err)
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/stripe/pay", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", user.Id)
	checkoutCalled := false

	subscriptionRequestStripePayWithCheckout(c, func(referenceID, customerID, email, priceID string) (string, error) {
		checkoutCalled = true
		var order model.SubscriptionOrder
		if err := model.DB.Where("trade_no = ?", referenceID).First(&order).Error; err != nil {
			t.Fatalf("pending subscription order did not exist before checkout: %v", err)
		}
		if order.Status != common.TopUpStatusPending || order.PaymentProvider != model.PaymentProviderStripe {
			t.Fatalf("subscription order before checkout = status %q provider %q", order.Status, order.PaymentProvider)
		}
		return "https://checkout.example.test/subscription", nil
	})

	if !checkoutCalled || !strings.Contains(response.Body.String(), `"message":"success"`) {
		t.Fatalf("expected successful subscription checkout, called=%v response=%s", checkoutCalled, response.Body.String())
	}
}

func TestStripeSubscriptionMarksOrderFailedWhenCheckoutFails(t *testing.T) {
	setupControllerAuditTestDB(t)
	configureStripeSubscriptionAuditTest(t)
	user := createPaymentAuditUser(t)
	plan := createStripeSubscriptionAuditPlan(t)
	body, err := common.Marshal(SubscriptionStripePayRequest{PlanId: plan.Id})
	if err != nil {
		t.Fatalf("failed to marshal subscription request: %v", err)
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/stripe/pay", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", user.Id)
	const upstreamSecret = "stripe subscription upstream failure detail"

	subscriptionRequestStripePayWithCheckout(c, func(referenceID, customerID, email, priceID string) (string, error) {
		return "", errors.New(upstreamSecret)
	})

	var order model.SubscriptionOrder
	if err := model.DB.First(&order).Error; err != nil {
		t.Fatalf("failed to read subscription order after checkout failure: %v", err)
	}
	if order.Status != common.TopUpStatusFailed {
		t.Fatalf("subscription order status = %q, want %q", order.Status, common.TopUpStatusFailed)
	}
	if !strings.Contains(response.Body.String(), "拉起支付失败") || strings.Contains(response.Body.String(), upstreamSecret) {
		t.Fatalf("subscription checkout error was not sanitized: %s", response.Body.String())
	}
}
