package ali

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

// https://help.aliyun.com/document_detail/613695.html?spm=a2c4g.2399480.0.0.1adb778fAdzP9w#341800c0f8w0r

const EnableSearchModelSuffix = "-internet"

func requestOpenAI2Ali(request dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	// DashScope rejects the 0 and 1 boundaries and some models accept at most
	// two decimal places. Omitted top_p must retain the model's default.
	if request.TopP != nil {
		if *request.TopP >= 1 {
			request.TopP = lo.ToPtr(0.99)
		} else if *request.TopP <= 0 {
			request.TopP = lo.ToPtr(0.01)
		}
	}
	return &request
}
