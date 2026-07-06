package controller

import "testing"

func TestExtractEpayPaidAmountUsesMoneyOnly(t *testing.T) {
	amount, ok := extractEpayPaidAmount(map[string]string{
		"money":  "12.34",
		"amount": "999.00",
	})
	if !ok {
		t.Fatal("expected epay paid amount to parse")
	}
	if amount != 12.34 {
		t.Fatalf("expected money field to win, got %.2f", amount)
	}
}

func TestExtractEpayPaidAmountRejectsGuessedFields(t *testing.T) {
	if _, ok := extractEpayPaidAmount(map[string]string{"amount": "12.34"}); ok {
		t.Fatal("amount field must not be guessed for epay")
	}
}
