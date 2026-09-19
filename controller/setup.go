package controller

import (
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Setup struct {
	Status       bool   `json:"status"`
	RootInit     bool   `json:"root_init"`
	DatabaseType string `json:"database_type"`
}

type SetupRequest struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	ConfirmPassword    string `json:"confirmPassword"`
	SelfUseModeEnabled bool   `json:"SelfUseModeEnabled"`
	DemoSiteEnabled    bool   `json:"DemoSiteEnabled"`
}

var postSetupMutex sync.Mutex

func GetSetup(c *gin.Context) {
	setup := Setup{
		Status: constant.Setup,
	}
	if constant.Setup {
		c.JSON(200, gin.H{
			"success": true,
			"data":    setup,
		})
		return
	}
	setup.RootInit = model.RootUserExists()
	if common.UsingMySQL {
		setup.DatabaseType = "mysql"
	}
	if common.UsingPostgreSQL {
		setup.DatabaseType = "postgres"
	}
	if common.UsingSQLite {
		setup.DatabaseType = "sqlite"
	}
	c.JSON(200, gin.H{
		"success": true,
		"data":    setup,
	})
}

func PostSetup(c *gin.Context) {
	postSetupMutex.Lock()
	defer postSetupMutex.Unlock()

	if constant.Setup {
		c.JSON(200, gin.H{
			"success": false,
			"message": "系统已经初始化完成",
		})
		return
	}

	var req SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": "请求参数有误",
		})
		return
	}

	var rootUser *model.User
	if !model.RootUserExists() {
		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" {
			c.JSON(200, gin.H{
				"success": false,
				"message": "用户名不能为空",
			})
			return
		}
		if len(req.Username) > 12 {
			c.JSON(200, gin.H{
				"success": false,
				"message": "用户名长度不能超过12个字符",
			})
			return
		}
		if req.Password != req.ConfirmPassword {
			c.JSON(200, gin.H{
				"success": false,
				"message": "两次输入的密码不一致",
			})
			return
		}

		if len(req.Password) < 8 {
			c.JSON(200, gin.H{
				"success": false,
				"message": "密码长度至少为8个字符",
			})
			return
		}

		hashedPassword, err := common.Password2Hash(req.Password)
		if err != nil {
			common.SysLog("failed to hash setup root password: " + err.Error())
			c.JSON(200, gin.H{
				"success": false,
				"message": "系统错误",
			})
			return
		}
		rootUser = &model.User{
			Username:    req.Username,
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
	}

	selfUseValue := boolToString(req.SelfUseModeEnabled)
	demoSiteValue := boolToString(req.DemoSiteEnabled)
	setupRecord := model.Setup{
		ID:            1,
		Version:       common.Version,
		InitializedAt: time.Now().Unix(),
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var setupCount int64
		if err := tx.Model(&model.Setup{}).Count(&setupCount).Error; err != nil {
			return err
		}
		if setupCount > 0 {
			return gorm.ErrDuplicatedKey
		}

		if err := tx.Create(&setupRecord).Error; err != nil {
			return err
		}

		var rootCount int64
		if err := tx.Model(&model.User{}).
			Where("role = ?", common.RoleRootUser).
			Count(&rootCount).Error; err != nil {
			return err
		}
		if rootCount == 0 {
			if rootUser == nil {
				return gorm.ErrInvalidData
			}
			if err := tx.Create(rootUser).Error; err != nil {
				return err
			}
		}

		options := []model.Option{
			{Key: "SelfUseModeEnabled", Value: selfUseValue},
			{Key: "DemoSiteEnabled", Value: demoSiteValue},
		}
		for i := range options {
			if err := tx.Save(&options[i]).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		common.SysLog("failed to commit initial setup transaction: " + err.Error())
		c.JSON(200, gin.H{
			"success": false,
			"message": "系统初始化失败",
		})
		return
	}

	operation_setting.SelfUseModeEnabled = req.SelfUseModeEnabled
	operation_setting.DemoSiteEnabled = req.DemoSiteEnabled

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["SelfUseModeEnabled"] = selfUseValue
	common.OptionMap["DemoSiteEnabled"] = demoSiteValue
	common.OptionMapRWMutex.Unlock()

	constant.Setup = true

	c.JSON(200, gin.H{
		"success": true,
		"message": "系统初始化成功",
	})
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
