package skill

import (
	"fmt"
	"strings"
	"time"
)

// ValidateServeOptions checks security and timing settings before starting any
// services. A zero callback budget selects the default for Go callers.
func ValidateServeOptions(skillToken string, callbackBudget time.Duration) error {
	if strings.TrimSpace(skillToken) == "" {
		return fmt.Errorf("--skill-token or KAKAO_SKILL_TOKEN is required")
	}
	if skillToken != strings.TrimSpace(skillToken) {
		return fmt.Errorf("skill token must not contain surrounding whitespace")
	}
	if callbackBudget < 0 || callbackBudget > defaultCallbackProcessingBudget {
		return fmt.Errorf("callback processing budget must be between 0 and %s", defaultCallbackProcessingBudget)
	}
	return nil
}
