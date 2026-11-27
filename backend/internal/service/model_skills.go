package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SettingKeyModelSkills           = "model_skills"
	SettingKeyGlobalModelSkillRules = "global_model_skill_rules"
	modelSkillsCacheTTL             = 30 * time.Second
	modelSkillsDBTimeout            = 5 * time.Second
)

// ModelSkillRuleConfig stores the model-to-Skill rules used by global settings
// and groups. A model named "*" matches all requested models.
type ModelSkillRuleConfig struct {
	DefaultSkillIDs []string            `json:"default_skill_ids"`
	ModelSkillIDs   map[string][]string `json:"model_skill_ids"`
}

// ModelSkill 是管理员维护的可复用模型技能说明。正文按 UTF-8 文本保存，
// 分组只保存 ID，便于修改技能内容后所有引用自动生效。
type ModelSkill struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Filename  string    `json:"filename,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cachedModelSkills struct {
	skills    []ModelSkill
	expiresAt int64
}

type cachedModelSkillRules struct {
	config    ModelSkillRuleConfig
	expiresAt int64
}

const maxModelSkillContentBytes = 512 * 1024

func (s *SettingService) GetModelSkills(ctx context.Context) ([]ModelSkill, error) {
	skills, err := s.getModelSkillsCached(ctx)
	if err != nil {
		return nil, err
	}
	return cloneModelSkills(skills), nil
}

func cloneModelSkills(skills []ModelSkill) []ModelSkill {
	if len(skills) == 0 {
		return []ModelSkill{}
	}
	return append([]ModelSkill(nil), skills...)
}

func (s *SettingService) getModelSkillsCached(ctx context.Context) ([]ModelSkill, error) {
	if s == nil || s.settingRepo == nil {
		return []ModelSkill{}, nil
	}
	now := time.Now().UnixNano()
	if cached, ok := s.modelSkillsCache.Load().(*cachedModelSkills); ok && cached != nil && now < cached.expiresAt {
		return cached.skills, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	generation := s.modelSkillsCacheGeneration.Load()
	result, err, _ := s.modelSkillsCacheSF.Do(SettingKeyModelSkills, func() (any, error) {
		if cached, ok := s.modelSkillsCache.Load().(*cachedModelSkills); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
			return cached.skills, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), modelSkillsDBTimeout)
		defer cancel()
		raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyModelSkills)
		if err != nil {
			if errors.Is(err, ErrSettingNotFound) {
				raw = ""
			} else {
				return nil, err
			}
		}
		var skills []ModelSkill
		if strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), &skills); err != nil {
				return nil, fmt.Errorf("decode model skills: %w", err)
			}
		}
		if skills == nil {
			skills = []ModelSkill{}
		}
		if s.modelSkillsCacheGeneration.Load() == generation {
			s.modelSkillsCache.Store(&cachedModelSkills{skills: skills, expiresAt: time.Now().Add(modelSkillsCacheTTL).UnixNano()})
		}
		return skills, nil
	})
	if err != nil {
		return nil, err
	}
	if skills, ok := result.([]ModelSkill); ok {
		return skills, nil
	}
	return []ModelSkill{}, nil
}

func (s *SettingService) invalidateModelSkillsCache() {
	if s == nil {
		return
	}
	s.modelSkillsCacheGeneration.Add(1)
	s.modelSkillsCacheSF.Forget(SettingKeyModelSkills)
	s.modelSkillsCache.Store(&cachedModelSkills{expiresAt: 0})
}

func (s *SettingService) saveModelSkills(ctx context.Context, skills []ModelSkill) error {
	data, err := json.Marshal(skills)
	if err != nil {
		return err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyModelSkills, string(data)); err != nil {
		return err
	}
	s.invalidateModelSkillsCache()
	return nil
}

func normalizeModelSkillIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeModelSkillRuleConfig(input ModelSkillRuleConfig) (ModelSkillRuleConfig, error) {
	modelSkillIDs, err := normalizeGroupModelSkillIDs(input.ModelSkillIDs)
	if err != nil {
		return ModelSkillRuleConfig{}, err
	}
	return ModelSkillRuleConfig{
		DefaultSkillIDs: normalizeModelSkillIDs(input.DefaultSkillIDs),
		ModelSkillIDs:   modelSkillIDs,
	}, nil
}

// GetGlobalModelSkillRules returns the rules configured under System Settings.
func (s *SettingService) GetGlobalModelSkillRules(ctx context.Context) (ModelSkillRuleConfig, error) {
	config, err := s.getGlobalModelSkillRulesCached(ctx)
	if err != nil {
		return ModelSkillRuleConfig{}, err
	}
	return cloneModelSkillRuleConfig(config), nil
}

func emptyModelSkillRuleConfig() ModelSkillRuleConfig {
	return ModelSkillRuleConfig{ModelSkillIDs: map[string][]string{}}
}

func cloneModelSkillRuleConfig(config ModelSkillRuleConfig) ModelSkillRuleConfig {
	cloned := ModelSkillRuleConfig{
		DefaultSkillIDs: append([]string(nil), config.DefaultSkillIDs...),
		ModelSkillIDs:   make(map[string][]string, len(config.ModelSkillIDs)),
	}
	for model, ids := range config.ModelSkillIDs {
		cloned.ModelSkillIDs[model] = append([]string(nil), ids...)
	}
	return cloned
}

func (s *SettingService) getGlobalModelSkillRulesCached(ctx context.Context) (ModelSkillRuleConfig, error) {
	if s == nil || s.settingRepo == nil {
		return emptyModelSkillRuleConfig(), nil
	}
	now := time.Now().UnixNano()
	if cached, ok := s.globalModelSkillRulesCache.Load().(*cachedModelSkillRules); ok && cached != nil && now < cached.expiresAt {
		return cached.config, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	generation := s.globalModelSkillRulesGeneration.Load()
	result, err, _ := s.globalModelSkillRulesCacheSF.Do(SettingKeyGlobalModelSkillRules, func() (any, error) {
		if cached, ok := s.globalModelSkillRulesCache.Load().(*cachedModelSkillRules); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
			return cached.config, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), modelSkillsDBTimeout)
		defer cancel()
		raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyGlobalModelSkillRules)
		if err != nil {
			if errors.Is(err, ErrSettingNotFound) {
				raw = ""
			} else {
				return nil, err
			}
		}
		config := emptyModelSkillRuleConfig()
		if strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), &config); err != nil {
				return nil, fmt.Errorf("decode global model skill rules: %w", err)
			}
			normalized, err := normalizeModelSkillRuleConfig(config)
			if err != nil {
				return nil, err
			}
			config = normalized
		}
		if s.globalModelSkillRulesGeneration.Load() == generation {
			s.globalModelSkillRulesCache.Store(&cachedModelSkillRules{config: config, expiresAt: time.Now().Add(modelSkillsCacheTTL).UnixNano()})
		}
		return config, nil
	})
	if err != nil {
		return ModelSkillRuleConfig{}, err
	}
	if config, ok := result.(ModelSkillRuleConfig); ok {
		return config, nil
	}
	return emptyModelSkillRuleConfig(), nil
}

func (s *SettingService) invalidateGlobalModelSkillRulesCache() {
	if s == nil {
		return
	}
	s.globalModelSkillRulesGeneration.Add(1)
	s.globalModelSkillRulesCacheSF.Forget(SettingKeyGlobalModelSkillRules)
	s.globalModelSkillRulesCache.Store(&cachedModelSkillRules{expiresAt: 0})
}

// UpdateGlobalModelSkillRules replaces the global model-to-Skill rules.
func (s *SettingService) UpdateGlobalModelSkillRules(ctx context.Context, input ModelSkillRuleConfig) (ModelSkillRuleConfig, error) {
	config, err := normalizeModelSkillRuleConfig(input)
	if err != nil {
		return ModelSkillRuleConfig{}, err
	}
	if s == nil || s.settingRepo == nil {
		return config, nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return ModelSkillRuleConfig{}, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyGlobalModelSkillRules, string(data)); err != nil {
		return ModelSkillRuleConfig{}, err
	}
	s.invalidateGlobalModelSkillRulesCache()
	return config, nil
}

func newModelSkillID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func normalizeModelSkillInput(name, filename, content string) (string, string, string, error) {
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	if name == "" {
		return "", "", "", errors.New("skill name is required")
	}
	if content == "" {
		return "", "", "", errors.New("skill content is required")
	}
	if len([]byte(content)) > maxModelSkillContentBytes {
		return "", "", "", fmt.Errorf("skill content cannot exceed %d KB", maxModelSkillContentBytes/1024)
	}
	return name, strings.TrimSpace(filename), content, nil
}

func (s *SettingService) CreateModelSkill(ctx context.Context, name, filename, content string) (*ModelSkill, error) {
	name, filename, content, err := normalizeModelSkillInput(name, filename, content)
	if err != nil {
		return nil, err
	}
	skills, err := s.GetModelSkills(ctx)
	if err != nil {
		return nil, err
	}
	id, err := newModelSkillID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	skill := ModelSkill{ID: id, Name: name, Filename: filename, Content: content, CreatedAt: now, UpdatedAt: now}
	skills = append(skills, skill)
	if err := s.saveModelSkills(ctx, skills); err != nil {
		return nil, err
	}
	return &skill, nil
}

func (s *SettingService) UpdateModelSkill(ctx context.Context, id, name, filename, content string) (*ModelSkill, error) {
	name, filename, content, err := normalizeModelSkillInput(name, filename, content)
	if err != nil {
		return nil, err
	}
	skills, err := s.GetModelSkills(ctx)
	if err != nil {
		return nil, err
	}
	for i := range skills {
		if skills[i].ID != id {
			continue
		}
		skills[i].Name, skills[i].Filename, skills[i].Content = name, filename, content
		skills[i].UpdatedAt = time.Now().UTC()
		if err := s.saveModelSkills(ctx, skills); err != nil {
			return nil, err
		}
		return &skills[i], nil
	}
	return nil, ErrSettingNotFound
}

func (s *SettingService) DeleteModelSkill(ctx context.Context, id string) error {
	skills, err := s.GetModelSkills(ctx)
	if err != nil {
		return err
	}
	filtered := skills[:0]
	found := false
	for _, skill := range skills {
		if skill.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, skill)
	}
	if !found {
		return ErrSettingNotFound
	}
	return s.saveModelSkills(ctx, filtered)
}
