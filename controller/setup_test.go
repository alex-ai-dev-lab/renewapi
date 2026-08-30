package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupControllerSetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Option{}, &model.Setup{}); err != nil {
		t.Fatalf("failed to migrate setup tables: %v", err)
	}
	model.DB = db

	previousSetup := constant.Setup
	previousSelfUse := operation_setting.SelfUseModeEnabled
	previousDemo := operation_setting.DemoSiteEnabled
	constant.Setup = false
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	previousSelfUseOption, hadSelfUseOption := common.OptionMap["SelfUseModeEnabled"]
	previousDemoOption, hadDemoOption := common.OptionMap["DemoSiteEnabled"]
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		constant.Setup = previousSetup
		operation_setting.SelfUseModeEnabled = previousSelfUse
		operation_setting.DemoSiteEnabled = previousDemo
		common.OptionMapRWMutex.Lock()
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
		common.OptionMapRWMutex.Unlock()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func postSetupRequest(t *testing.T, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(resp)
	ctx.Request = req
	PostSetup(ctx)
	return resp
}

func TestPostSetupRejectsBlankRootUsername(t *testing.T) {
	setupControllerSetupTestDB(t)

	resp := postSetupRequest(t, map[string]any{
		"username":           "   ",
		"password":           "password123",
		"confirmPassword":    "password123",
		"SelfUseModeEnabled": true,
		"DemoSiteEnabled":    false,
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "用户名不能为空") {
		t.Fatalf("expected blank username error, got %s", resp.Body.String())
	}
	var count int64
	if err := model.DB.Model(&model.User{}).Where("role = ?", common.RoleRootUser).Count(&count).Error; err != nil {
		t.Fatalf("failed to count roots: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no root user, got %d", count)
	}
}

func TestPostSetupCreatesSingletonRootAndDurableOptions(t *testing.T) {
	setupControllerSetupTestDB(t)

	resp := postSetupRequest(t, map[string]any{
		"username":           "rootadmin",
		"password":           "password123",
		"confirmPassword":    "password123",
		"SelfUseModeEnabled": true,
		"DemoSiteEnabled":    false,
	})
	if !strings.Contains(resp.Body.String(), `"success":true`) {
		t.Fatalf("expected setup success, got %s", resp.Body.String())
	}

	var setupCount int64
	if err := model.DB.Model(&model.Setup{}).Count(&setupCount).Error; err != nil {
		t.Fatalf("failed to count setup rows: %v", err)
	}
	if setupCount != 1 {
		t.Fatalf("expected one setup row, got %d", setupCount)
	}
	var setup model.Setup
	if err := model.DB.First(&setup).Error; err != nil {
		t.Fatalf("failed to read setup row: %v", err)
	}
	if setup.ID != 1 {
		t.Fatalf("expected singleton setup id 1, got %d", setup.ID)
	}

	var rootCount int64
	if err := model.DB.Model(&model.User{}).Where("role = ?", common.RoleRootUser).Count(&rootCount).Error; err != nil {
		t.Fatalf("failed to count roots: %v", err)
	}
	if rootCount != 1 {
		t.Fatalf("expected one root user, got %d", rootCount)
	}

	var selfUse model.Option
	if err := model.DB.First(&selfUse, "key = ?", "SelfUseModeEnabled").Error; err != nil {
		t.Fatalf("failed to read SelfUseModeEnabled: %v", err)
	}
	if selfUse.Value != "true" {
		t.Fatalf("expected SelfUseModeEnabled=true, got %q", selfUse.Value)
	}
	if !constant.Setup {
		t.Fatal("expected in-process setup flag after durable commit")
	}
}
