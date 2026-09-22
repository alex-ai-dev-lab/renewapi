package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetChannelTestPrompts(c *gin.Context) {
	prompts, err := model.ListChannelTestPrompts()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, prompts)
}

func CreateChannelTestPrompt(c *gin.Context) { saveChannelTestPrompt(c, true) }
func UpdateChannelTestPrompt(c *gin.Context) { saveChannelTestPrompt(c, false) }

func saveChannelTestPrompt(c *gin.Context, create bool) {
	var input struct {
		Name        string `json:"name"`
		Prompt      string `json:"prompt"`
		Description string `json:"description"`
		Enabled     *bool  `json:"enabled"`
		IsDefault   bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	prompt := model.ChannelTestPrompt{Name: input.Name, Prompt: input.Prompt, Description: input.Description, Enabled: true, IsDefault: input.IsDefault}
	if input.Enabled != nil {
		prompt.Enabled = *input.Enabled
	}
	if !create {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			common.ApiErrorMsg(c, "无效的提示词 ID")
			return
		}
		prompt.ID = id
	}
	if err := model.SaveChannelTestPrompt(&prompt, create); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, prompt)
}

func DeleteChannelTestPrompt(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提示词 ID")
		return
	}
	if err := model.DeleteChannelTestPrompt(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
