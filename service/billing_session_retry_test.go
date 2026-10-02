package service

import "testing"

func TestBillingSessionCanRetryRequestUsesLifecycleNotReservationAmount(t *testing.T) {
	for _, tc := range []struct {
		name    string
		session *BillingSession
		want    bool
	}{
		{"active prepaid", &BillingSession{preConsumedQuota: 100, tokenConsumed: 100}, true},
		{"active trusted zero", &BillingSession{trusted: true}, true},
		{"settled", &BillingSession{settled: true, preConsumedQuota: 100}, false},
		{"funding settled", &BillingSession{fundingSettled: true, preConsumedQuota: 100}, false},
		{"refunded", &BillingSession{refunded: true, preConsumedQuota: 100}, false},
		{"pending async task", &BillingSession{pendingTaskID: 1, preConsumedQuota: 100}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.session.CanRetryRequest(); got != tc.want {
				t.Fatalf("CanRetryRequest()=%v, want %v", got, tc.want)
			}
		})
	}
}
