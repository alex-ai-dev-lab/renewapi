package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedeemCreditsQuotaExactlyOnce(t *testing.T) {
	truncateTables(t)
	user := User{Username: "redeem-user", Password: "password", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	redemption := Redemption{
		Name:        "redeem-test",
		Key:         "10000000000000000000000000000001",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       300,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&redemption).Error)

	const attempts = 5
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Redeem(redemption.Key, user.Id)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	assert.Equal(t, 1, successes)
	require.NoError(t, DB.First(&user, user.Id).Error)
	assert.Equal(t, 300, user.Quota)
}
