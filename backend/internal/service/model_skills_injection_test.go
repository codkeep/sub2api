//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type modelSkillsProbeSettingRepo struct {
	SettingRepository
	values map[string]string
	reads  int
}

func (r *modelSkillsProbeSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.reads++
	return r.values[key], nil
}

func (r *modelSkillsProbeSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func TestModelSkillsChannelMonitorProbeIsolation(t *testing.T) {
	protocols := map[string]string{
		"anthropic":        `{"model":"test-model","system":"original instructions","messages":[{"role":"user","content":"hi"}]}`,
		"chat_completions": `{"model":"test-model","messages":[{"role":"system","content":"original instructions"},{"role":"user","content":"hi"}]}`,
		"responses":        `{"model":"test-model","instructions":"original instructions","input":"hi"}`,
		"gemini":           `{"systemInstruction":{"parts":[{"text":"original instructions"}]},"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`,
	}
	for _, rules := range []struct {
		name, requestedModel, globalID, groupID string
		modelRules                              bool
	}{
		{"defaults", "test-model", "global", "group", false},
		{"model-specific", "test-model", "global-model", "group-model", true},
		{"wildcard", "other-model", "global-wildcard", "group-wildcard", true},
	} {
		for protocol, raw := range protocols {
			t.Run(rules.name+"/"+protocol, func(t *testing.T) {
				global := ModelSkillRuleConfig{DefaultSkillIDs: []string{"shared", "global"}}
				group := &Group{DefaultSkillIDs: []string{"shared", "group"}}
				if rules.modelRules {
					global.ModelSkillIDs = map[string][]string{"test-model": {"shared", "global-model"}, "*": {"shared", "global-wildcard"}}
					group.ModelSkillIDs = map[string][]string{"test-model": {"shared", "group-model"}, "*": {"shared", "group-wildcard"}}
				}
				var skills []ModelSkill
				for _, id := range []string{"shared", "global", "group", "global-model", "group-model", "global-wildcard", "group-wildcard"} {
					skills = append(skills, ModelSkill{ID: id, Name: id, Content: "skill-content[" + id + "]"})
				}
				globalJSON, err := json.Marshal(global)
				require.NoError(t, err)
				skillsJSON, err := json.Marshal(skills)
				require.NoError(t, err)
				repo := &modelSkillsProbeSettingRepo{values: map[string]string{
					SettingKeyGlobalModelSkillRules: string(globalJSON), SettingKeyModelSkills: string(skillsJSON),
				}}
				settings := &SettingService{settingRepo: repo}
				body := []byte(raw)
				base := context.Background()
				probe := WithChannelMonitorProbe(base)
				for _, inject := range []func(context.Context, *Group, string, string, []byte) ([]byte, error){
					settings.ApplyGroupDefaultSkills,
					(&GatewayService{settingService: settings}).ApplyGroupDefaultSkills,
					(&OpenAIGatewayService{settingService: settings}).ApplyGroupDefaultSkills,
				} {
					actual, err := inject(probe, group, protocol, rules.requestedModel, body)
					require.NoError(t, err)
					require.Equal(t, body, actual, "probe body must remain byte-for-byte unchanged")
				}
				require.Zero(t, repo.reads, "probes must not depend on Skill settings storage")
				require.False(t, IsChannelMonitorProbe(base), "the parent context must remain a normal request")

				normal, err := settings.ApplyGroupDefaultSkills(base, group, protocol, rules.requestedModel, body)
				require.NoError(t, err)
				require.Contains(t, string(normal), "original instructions")
				for _, skill := range skills {
					count := strings.Count(string(normal), skill.Content)
					if skill.ID == "shared" || skill.ID == rules.globalID || skill.ID == rules.groupID {
						require.Equal(t, 1, count, "selected global/group Skills must still be merged and deduplicated: %s", skill.ID)
					} else {
						require.Zero(t, count, "model rules must not leak into other models: %s", skill.ID)
					}
				}
				again, err := settings.ApplyGroupDefaultSkills(base, group, protocol, rules.requestedModel, normal)
				require.NoError(t, err)
				require.Equal(t, string(normal), string(again), "existing Skill injection deduplication must remain intact")
			})
		}
	}
}

func TestModelSkillsChannelMonitorProbeSkipsInvalidSettings(t *testing.T) {
	repo := &modelSkillsProbeSettingRepo{values: map[string]string{SettingKeyGlobalModelSkillRules: "invalid JSON"}}
	settings := &SettingService{settingRepo: repo}
	body := []byte(`{"model":"test-model","input":"hi"}`)
	actual, err := settings.ApplyGroupDefaultSkills(WithChannelMonitorProbe(context.Background()), nil, "responses", "test-model", body)
	require.NoError(t, err)
	require.Equal(t, body, actual)
	require.Zero(t, repo.reads)
	_, err = settings.ApplyGroupDefaultSkills(context.Background(), nil, "responses", "test-model", body)
	require.Error(t, err, "ordinary requests must still load configured Skills")
}

func TestModelSkillsCacheAvoidsRepeatedReadsAndInvalidatesAfterWrite(t *testing.T) {
	global, err := json.Marshal(ModelSkillRuleConfig{DefaultSkillIDs: []string{"global"}})
	require.NoError(t, err)
	skills, err := json.Marshal([]ModelSkill{{ID: "global", Name: "global", Content: "cached skill"}})
	require.NoError(t, err)
	repo := &modelSkillsProbeSettingRepo{values: map[string]string{
		SettingKeyGlobalModelSkillRules: string(global),
		SettingKeyModelSkills:           string(skills),
	}}
	settings := &SettingService{settingRepo: repo}
	group := &Group{ID: 42}
	firstBody := []byte(`{"model":"test-model","instructions":"first"}`)
	secondBody := []byte(`{"model":"test-model","instructions":"second"}`)

	first, err := settings.ApplyGroupDefaultSkills(context.Background(), group, "responses", "test-model", firstBody)
	require.NoError(t, err)
	require.Contains(t, string(first), "cached skill")
	readsAfterFirst := repo.reads
	require.Equal(t, 2, readsAfterFirst, "the first request loads each settings document once")

	second, err := settings.ApplyGroupDefaultSkills(context.Background(), group, "responses", "test-model", secondBody)
	require.NoError(t, err)
	require.Contains(t, string(second), "cached skill")
	require.Equal(t, readsAfterFirst, repo.reads, "subsequent requests should use the in-process cache")

	_, err = settings.UpdateGlobalModelSkillRules(context.Background(), ModelSkillRuleConfig{})
	require.NoError(t, err)
	third, err := settings.ApplyGroupDefaultSkills(context.Background(), group, "responses", "test-model", secondBody)
	require.NoError(t, err)
	require.NotContains(t, string(third), "cached skill", "a successful admin write must invalidate the cache immediately")
	require.Greater(t, repo.reads, readsAfterFirst)
}

func BenchmarkApplyGroupDefaultSkillsCached(b *testing.B) {
	global, _ := json.Marshal(ModelSkillRuleConfig{DefaultSkillIDs: []string{"global"}})
	skills, _ := json.Marshal([]ModelSkill{{ID: "global", Name: "global", Content: "cached skill"}})
	repo := &modelSkillsProbeSettingRepo{values: map[string]string{
		SettingKeyGlobalModelSkillRules: string(global),
		SettingKeyModelSkills:           string(skills),
	}}
	settings := &SettingService{settingRepo: repo}
	group := &Group{ID: 42}
	body := []byte(`{"model":"test-model","instructions":"benchmark"}`)
	_, _ = settings.ApplyGroupDefaultSkills(context.Background(), group, "responses", "test-model", body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = settings.ApplyGroupDefaultSkills(context.Background(), group, "responses", "test-model", body)
	}
}
