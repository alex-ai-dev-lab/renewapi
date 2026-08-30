package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSettleLegacyBillingBalancesExhaustsFiniteWalletAndTokenAfterOverage(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", fixture.user.Id).Update("quota", 50).Error)
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Updates(map[string]any{
		"remain_quota": 50,
		"used_quota":   100,
	}).Error)

	err := SettleLegacyBillingBalances(LegacyBillingSettlement{
		FundingSource: "wallet",
		UserID:        fixture.user.Id,
		TokenID:       fixture.token.Id,
		Delta:         100,
	})
	require.NoError(t, err)

	var user User
	var token Token
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.NoError(t, DB.First(&token, fixture.token.Id).Error)
	require.Zero(t, user.Quota)
	require.Zero(t, token.RemainQuota)
	require.Equal(t, 200, token.UsedQuota)
}

func TestSettleLegacyBillingBalancesRollsBackFundingWhenTokenUpdateFails(t *testing.T) {
	fixture := setupBillingLedgerTest(t)

	err := SettleLegacyBillingBalances(LegacyBillingSettlement{
		FundingSource: "wallet",
		UserID:        fixture.user.Id,
		TokenID:       fixture.token.Id + 9999,
		Delta:         100,
	})
	require.ErrorContains(t, err, "token post-usage settlement invariant failed")

	var user User
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.Equal(t, 1000, user.Quota)
}

func TestSettleLegacyBillingBalancesRefundsFundingAndTokenTogether(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", fixture.user.Id).Update("quota", 900).Error)
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Updates(map[string]any{
		"remain_quota": 900,
		"used_quota":   100,
	}).Error)

	require.NoError(t, SettleLegacyBillingBalances(LegacyBillingSettlement{
		FundingSource: "wallet",
		UserID:        fixture.user.Id,
		TokenID:       fixture.token.Id,
		Delta:         -40,
	}))

	var user User
	var token Token
	require.NoError(t, DB.First(&user, fixture.user.Id).Error)
	require.NoError(t, DB.First(&token, fixture.token.Id).Error)
	require.Equal(t, 940, user.Quota)
	require.Equal(t, 940, token.RemainQuota)
	require.Equal(t, 60, token.UsedQuota)
}

func TestSettleLegacyBillingBalancesExhaustsFiniteSubscriptionAndTokenAfterOverage(t *testing.T) {
	fixture := setupBillingLedgerTest(t)
	now := time.Now().Unix()
	sub := UserSubscription{
		UserId:      fixture.user.Id,
		PlanId:      1,
		AmountTotal: 150,
		AmountUsed:  100,
		StartTime:   now - 60,
		EndTime:     now + 3600,
		Status:      "active",
		Source:      "test",
	}
	require.NoError(t, DB.Create(&sub).Error)
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", fixture.token.Id).Updates(map[string]any{
		"remain_quota": 25,
		"used_quota":   100,
	}).Error)

	require.NoError(t, SettleLegacyBillingBalances(LegacyBillingSettlement{
		FundingSource:  "subscription",
		UserID:         fixture.user.Id,
		SubscriptionID: sub.Id,
		TokenID:        fixture.token.Id,
		Delta:          75,
	}))

	var gotSub UserSubscription
	var token Token
	require.NoError(t, DB.First(&gotSub, sub.Id).Error)
	require.NoError(t, DB.First(&token, fixture.token.Id).Error)
	require.EqualValues(t, 150, gotSub.AmountUsed)
	require.Zero(t, token.RemainQuota)
	require.Equal(t, 175, token.UsedQuota)
}
