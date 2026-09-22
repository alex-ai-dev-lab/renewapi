package common

import (
	"strings"

	"github.com/tidwall/gjson"
)

// ResponseModel 仅供管理员排障，不改变路由、客户端模型名或计费模型。
type ResponseModel struct {
	RequestedModel string `json:"requested_model"`
	UpstreamModel  string `json:"upstream_model"`
	ReturnedModel  string `json:"returned_model"`
}

func (r *ResponseModel) Mismatch() bool {
	if r == nil || r.ReturnedModel == "" {
		return false
	}
	returned := strings.ToLower(r.ReturnedModel)
	for _, expected := range []string{r.RequestedModel, r.UpstreamModel} {
		expected = strings.ToLower(strings.TrimSpace(expected))
		if expected != "" && (returned == expected || returned[strings.LastIndex(returned, "/")+1:] == expected) {
			return false
		}
	}
	return true
}

func (info *RelayInfo) ObserveResponseModel(model string) {
	model = strings.TrimSpace(model)
	if info == nil || model == "" || len(model) > 256 {
		return
	}
	if info.ResponseModel == nil {
		info.ResponseModel = &ResponseModel{RequestedModel: info.ClientModel(), UpstreamModel: info.UpstreamModel()}
	}
	if !info.ResponseModel.Mismatch() {
		info.ResponseModel.ReturnedModel = model
	}
}

// ObserveResponseModelJSON 在协议转换前识别各协议的原生模型声明。
func (info *RelayInfo) ObserveResponseModelJSON(data string) {
	if !gjson.Valid(data) {
		return
	}
	for _, path := range []string{"model", "response.model", "message.model", "modelVersion"} {
		if value := gjson.Get(data, path); value.Type == gjson.String {
			info.ObserveResponseModel(value.String())
		}
	}
}
