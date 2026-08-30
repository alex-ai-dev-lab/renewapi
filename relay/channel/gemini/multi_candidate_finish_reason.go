package gemini

import "github.com/QuantumNous/new-api/dto"

func normalizeGeminiStreamChoiceFinishReasons(response *dto.GeminiChatResponse, converted *dto.ChatCompletionsStreamResponse) {
	if response == nil || converted == nil || len(response.Candidates) == 0 || len(converted.Choices) == 0 {
		return
	}
	candidates := make(map[int64]dto.GeminiChatCandidate, len(response.Candidates))
	for _, candidate := range response.Candidates {
		candidates[candidate.Index] = candidate
	}
	for i := range converted.Choices {
		candidate, ok := candidates[int64(converted.Choices[i].Index)]
		if !ok || candidate.FinishReason == nil {
			continue
		}
		finishReason := geminiCandidateOpenAIFinishReason(candidate)
		converted.Choices[i].FinishReason = &finishReason
	}
}
