package channelconfig

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

const crossingMapping = `{"version":2,"rules":[{"id":"a","from":"A","to":"B"},{"id":"b","from":"B","to":"A"},{"id":"disabled","from":"hidden","to":"upstream","enabled":false}]}`

func TestMappingWriteValidation(t *testing.T) {
	invalid := []string{`null`, `{`, `{"a":"b","b":"a"}`, `{"version":2,"rules":[{"id":"a","from":"a","to":"b","priority":1.5}]}`, `{"version":2,"rules":[{"id":"a","from":"a","to":"b","priority":2147483648}]}`}
	for _, raw := range invalid {
		t.Run(raw, func(t *testing.T) {
			db := setupChannelConfigTestDB(t)
			channel := createConfigTestChannel(t, db)
			channel.ModelMapping = &raw
			require.Error(t, ValidateChannel(&channel, true))
			_, err := Update(UpdateCommand{Channel: &channel, PresentFields: map[string]bool{"model_mapping": true}})
			require.Error(t, err)
			var stored model.Channel
			require.NoError(t, db.First(&stored, channel.Id).Error)
			require.Nil(t, stored.ModelMapping)
			var count int64
			require.NoError(t, db.Model(&model.ConfigAudit{}).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestMappingUpdatePreservesHistoricalValues(t *testing.T) {
	for _, raw := range []string{`null`, `{"a":"b","b":"a"}`, `{broken`} {
		t.Run(raw, func(t *testing.T) {
			db := setupChannelConfigTestDB(t)
			channel := createConfigTestChannel(t, db)
			require.NoError(t, db.Model(&channel).Update("model_mapping", raw).Error)
			channel.ModelMapping = &raw
			channel.Name = "renamed"
			result, err := Update(UpdateCommand{Channel: &channel, PresentFields: map[string]bool{"name": true}})
			require.NoError(t, err)
			require.Equal(t, raw, result.Channel.GetModelMapping())
		})
	}
}

func TestMappingRulesUpdateRoutingAndDowngradeProtection(t *testing.T) {
	db := setupChannelConfigTestDB(t)
	channel := createConfigTestChannel(t, db)
	raw := crossingMapping
	channel.ModelMapping = &raw
	require.NoError(t, ValidateChannel(&channel, true))
	expected := channel.ConfigVersion
	result, err := Update(UpdateCommand{Channel: &channel, ExpectedConfigVersion: &expected, PresentFields: map[string]bool{"model_mapping": true}})
	require.NoError(t, err)
	require.True(t, result.ChangeSet.AbilityChanged)
	require.True(t, result.ChangeSet.RoutingChanged)
	require.True(t, result.CacheSynchronized)
	require.ElementsMatch(t, []string{"gpt-test", "A", "B"}, result.Channel.GetRoutingModels())
	var abilities []model.Ability
	require.NoError(t, db.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.Len(t, abilities, 3)
	for _, next := range []string{`{"A":"B"}`, `{}`, ``} {
		proposed := *result.Channel
		proposed.ModelMapping = &next
		_, err := Update(UpdateCommand{Channel: &proposed, PresentFields: map[string]bool{"model_mapping": true}})
		require.Error(t, err)
	}
	clear := `{"version":2,"rules":[]}`
	proposed := *result.Channel
	proposed.ModelMapping = &clear
	_, err = Update(UpdateCommand{Channel: &proposed, ExpectedConfigVersion: &expected, PresentFields: map[string]bool{"model_mapping": true}})
	require.ErrorIs(t, err, model.ErrChannelConfigConflict)
	expected = result.Channel.ConfigVersion
	result, err = Update(UpdateCommand{Channel: &proposed, ExpectedConfigVersion: &expected, PresentFields: map[string]bool{"model_mapping": true}})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-test"}, result.Channel.GetRoutingModels())
}

func TestTagMappingValidationIsAtomic(t *testing.T) {
	for _, raw := range []string{`null`, `{`, `{"a":"b","b":"a"}`, `{"version":2,"rules":[{"id":"a","from":"a","to":"b","priority":1.5}]}`, `{"version":2,"rules":[{"id":"a","from":"a","to":"b","priority":2147483648}]}`, `{"a":"b"}`} {
		t.Run(raw, func(t *testing.T) {
			db := setupChannelConfigTestDB(t)
			first := createConfigTestChannel(t, db)
			second := createConfigTestChannel(t, db)
			tag, renamed := "batch", "renamed"
			require.NoError(t, db.Model(&model.Channel{}).Where("id IN ?", []int{first.Id, second.Id}).Update("tag", tag).Error)
			require.NoError(t, db.Model(&second).Update("model_mapping", crossingMapping).Error)
			err := model.EditChannelByTag(tag, &renamed, &raw, nil, nil, nil, nil, nil, nil)
			require.Error(t, err)
			require.NoError(t, db.First(&first, first.Id).Error)
			require.Equal(t, tag, first.GetTag())
			require.Nil(t, first.ModelMapping)
		})
	}
}

func TestTagMappingUpdatesAbilitiesAndPreservesEmptySemantics(t *testing.T) {
	db := setupChannelConfigTestDB(t)
	channel := createConfigTestChannel(t, db)
	tag := "batch"
	require.NoError(t, db.Model(&channel).Update("tag", tag).Error)
	raw := crossingMapping
	require.NoError(t, model.EditChannelByTag(tag, nil, &raw, nil, nil, nil, nil, nil, nil))
	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, raw, stored.GetModelMapping())
	require.Greater(t, stored.ConfigVersion, channel.ConfigVersion)
	var abilities []model.Ability
	require.NoError(t, db.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.Len(t, abilities, 3)
	empty := ""
	require.NoError(t, model.EditChannelByTag(tag, nil, &empty, nil, nil, nil, nil, nil, nil))
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, raw, stored.GetModelMapping())
	clear := `{"version":2,"rules":[]}`
	require.NoError(t, model.EditChannelByTag(tag, nil, &clear, nil, nil, nil, nil, nil, nil))
	abilities = nil
	require.NoError(t, db.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.Len(t, abilities, 1)
}

func TestTagMappingAbilityFailureRollsBackEntireBatch(t *testing.T) {
	db := setupChannelConfigTestDB(t)
	first := createConfigTestChannel(t, db)
	second := createConfigTestChannel(t, db)
	tag := "batch"
	require.NoError(t, db.Model(&model.Channel{}).Where("id IN ?", []int{first.Id, second.Id}).Update("tag", tag).Error)
	require.NoError(t, db.Exec(fmt.Sprintf(`CREATE TRIGGER fail_mapping_ability BEFORE INSERT ON abilities WHEN NEW.channel_id = %d BEGIN SELECT RAISE(FAIL, 'forced ability failure'); END`, second.Id)).Error)
	raw := crossingMapping
	require.ErrorContains(t, model.EditChannelByTag(tag, nil, &raw, nil, nil, nil, nil, nil, nil), "forced ability failure")
	for _, id := range []int{first.Id, second.Id} {
		var stored model.Channel
		require.NoError(t, db.First(&stored, id).Error)
		require.Nil(t, stored.ModelMapping)
		require.Equal(t, int64(1), stored.ConfigVersion)
		var abilities []model.Ability
		require.NoError(t, db.Where("channel_id = ?", id).Find(&abilities).Error)
		require.Len(t, abilities, 1)
	}
}

func TestMappingSourcesNeverExposeTargets(t *testing.T) {
	raw := `{"version":2,"rules":[{"id":"a","from":"alias","to":"private"},{"id":"b","from":"disabled","to":"other","enabled":false}]}`
	channel := model.Channel{Models: "explicit,disabled", ModelMapping: &raw}
	require.ElementsMatch(t, []string{"explicit", "disabled", "alias"}, channel.GetRoutingModels())
	channel.Models = ""
	channel.Name, channel.Group, channel.Key = "name", "default", "key"
	require.Error(t, ValidateChannel(&channel, true))
}
