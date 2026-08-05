package strategy

import (
	"strings"

	"github.com/shikanon/cookies/internal/systems/strategy/promptkit"
)

func (s Service) conversationPromptVersion() string {
	if value := strings.TrimSpace(s.ConversationPromptVersion); value != "" {
		return value
	}
	return promptkit.ConversationV5
}

func (s Service) generatePromptVersion() string {
	if value := strings.TrimSpace(s.PromptVersion); value != "" {
		return value
	}
	return promptkit.GenerateV3
}

func (s Service) revisePromptVersion() string {
	if value := strings.TrimSpace(s.RevisePromptVersion); value != "" {
		return value
	}
	return promptkit.ReviseV3
}

func (s Service) reviewPromptVersion() string {
	if value := strings.TrimSpace(s.ReviewPromptVersion); value != "" {
		return value
	}
	return promptkit.ReviewV2
}

func (s Service) repairPromptVersion() string {
	if value := strings.TrimSpace(s.RepairPromptVersion); value != "" {
		return value
	}
	return promptkit.RepairV2
}
