package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func normalizeGroupModelSkillIDs(input map[string][]string) (map[string][]string, error) {
	out := make(map[string][]string, len(input))
	for model, ids := range input {
		model = strings.TrimSpace(model)
		if model == "" {
			return nil, infraerrors.BadRequest("GROUP_SKILLS_MODEL_REQUIRED", "group Skills require a model name")
		}
		if _, exists := out[model]; exists {
			return nil, infraerrors.BadRequest("GROUP_SKILLS_MODEL_DUPLICATE", fmt.Sprintf("duplicate group Skills model %q", model))
		}
		seen := make(map[string]struct{}, len(ids))
		clean := make([]string, 0, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if _, exists := seen[id]; id == "" || exists {
				continue
			}
			seen[id] = struct{}{}
			clean = append(clean, id)
		}
		if len(clean) == 0 {
			return nil, infraerrors.BadRequest("GROUP_SKILLS_SELECTION_REQUIRED", fmt.Sprintf("select at least one Skill for model %q", model))
		}
		out[model] = clean
	}
	return out, nil
}

func cloneGroupModelSkillIDs(input map[string][]string) map[string][]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string][]string, len(input))
	for model, ids := range input {
		out[model] = append([]string(nil), ids...)
	}
	return out
}

// ApplyGroupDefaultSkills 将分组为客户端请求模型配置的 Skills 合并进系统级指令。
// Skills 只保存 ID，正文从系统设置中读取，因此修改 Skill 后所有引用它的分组会自动生效。
// protocol 支持 anthropic、chat_completions、responses 和 gemini 四种入口格式。
func (s *SettingService) ApplyGroupDefaultSkills(ctx context.Context, group *Group, protocol, requestedModel string, body []byte) ([]byte, error) {
	if s == nil || group == nil || len(body) == 0 {
		return body, nil
	}

	skillIDs := group.DefaultSkillIDs
	if len(group.ModelSkillIDs) > 0 {
		modelSkillIDs, matched := group.ModelSkillIDs[strings.TrimSpace(requestedModel)]
		if matched {
			skillIDs = modelSkillIDs
		}
	}
	if len(skillIDs) == 0 {
		return body, nil
	}

	skills, err := s.GetModelSkills(ctx)
	if err != nil {
		return body, fmt.Errorf("load model skills: %w", err)
	}
	byID := make(map[string]ModelSkill, len(skills))
	for _, skill := range skills {
		byID[skill.ID] = skill
	}

	var prompt strings.Builder
	for _, id := range skillIDs {
		skill, ok := byID[strings.TrimSpace(id)]
		if !ok || strings.TrimSpace(skill.Content) == "" {
			continue
		}
		if prompt.Len() == 0 {
			prompt.WriteString("以下是当前分组启用的默认 Skills。请将它们作为本次对话的工作规范，并在整个请求中遵循：\n\n")
		}
		prompt.WriteString("## ")
		prompt.WriteString(strings.TrimSpace(skill.Name))
		prompt.WriteString("\n")
		prompt.WriteString(strings.TrimSpace(skill.Content))
		prompt.WriteString("\n\n")
	}
	if prompt.Len() == 0 {
		return body, nil
	}

	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return body, fmt.Errorf("parse request for default skills: %w", err)
	}
	if root == nil {
		return body, nil
	}
	instruction := strings.TrimSpace(prompt.String())
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "anthropic":
		applyAnthropicSkillInstruction(root, instruction)
	case "chat_completions", "chat-completions", "chat":
		applyChatCompletionsSkillInstruction(root, instruction)
	case "gemini":
		applyGeminiSkillInstruction(root, instruction)
	case "responses", "openai":
		applyResponsesSkillInstruction(root, instruction)
	default:
		return body, nil
	}

	updated, err := json.Marshal(root)
	if err != nil {
		return body, fmt.Errorf("encode request with default skills: %w", err)
	}
	return updated, nil
}

// ApplyGroupDefaultSkills exposes the shared injection point to the gateway
// services while keeping settings storage details inside SettingService.
func (s *GatewayService) ApplyGroupDefaultSkills(ctx context.Context, group *Group, protocol, requestedModel string, body []byte) ([]byte, error) {
	if s == nil || s.settingService == nil {
		return body, nil
	}
	return s.settingService.ApplyGroupDefaultSkills(ctx, group, protocol, requestedModel, body)
}

// ApplyGroupDefaultSkills exposes the shared injection point to OpenAI-compatible
// handlers, including the native Responses and /v1/messages dispatch paths.
func (s *OpenAIGatewayService) ApplyGroupDefaultSkills(ctx context.Context, group *Group, protocol, requestedModel string, body []byte) ([]byte, error) {
	if s == nil || s.settingService == nil {
		return body, nil
	}
	return s.settingService.ApplyGroupDefaultSkills(ctx, group, protocol, requestedModel, body)
}

func appendSkillText(value any, instruction string) any {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return instruction
		}
		return typed + "\n\n" + instruction
	case []any:
		return append(typed, map[string]any{"type": "text", "text": instruction})
	default:
		return instruction
	}
}

func applyAnthropicSkillInstruction(root map[string]any, instruction string) {
	if existing, ok := root["system"]; ok && existing != nil {
		root["system"] = appendSkillText(existing, instruction)
		return
	}
	root["system"] = instruction
}

func applyChatCompletionsSkillInstruction(root map[string]any, instruction string) {
	messages, ok := root["messages"].([]any)
	if !ok {
		root["messages"] = []any{map[string]any{"role": "system", "content": instruction}}
		return
	}
	for _, item := range messages {
		message, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		if strings.EqualFold(role, "system") || strings.EqualFold(role, "developer") {
			message["content"] = appendSkillText(message["content"], instruction)
			return
		}
	}
	root["messages"] = append([]any{map[string]any{"role": "system", "content": instruction}}, messages...)
}

func applyResponsesSkillInstruction(root map[string]any, instruction string) {
	if existing, ok := root["instructions"]; ok && existing != nil {
		root["instructions"] = appendSkillText(existing, instruction)
		return
	}
	root["instructions"] = instruction
}

func applyGeminiSkillInstruction(root map[string]any, instruction string) {
	systemInstruction, ok := root["systemInstruction"].(map[string]any)
	if !ok || systemInstruction == nil {
		systemInstruction = map[string]any{}
		root["systemInstruction"] = systemInstruction
	}
	parts, _ := systemInstruction["parts"].([]any)
	systemInstruction["parts"] = append(parts, map[string]any{"text": instruction})
}
