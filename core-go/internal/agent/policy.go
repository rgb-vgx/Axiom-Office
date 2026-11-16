package agent

import (
	"strings"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/policy"
)

// policyRules: bo quy tac cua luot = policy.DefaultRules() roi den policy.rules trong cau hinh. Quy tac
// nguoi dung nam SAU nen thang mac dinh khi cung khop (last match wins). Effect la trong cau hinh bi bo
// qua (config.policyRules da chan quy tac thieu truong, day kiem tra lai cho chac).
func policyRules(cfg config.Config) policy.Ruleset {
	rules := policy.DefaultRules()
	for _, rule := range cfg.Policy {
		effect, ok := policy.ParseEffect(rule.Effect)
		if !ok || strings.TrimSpace(rule.Action) == "" {
			continue
		}
		rules = append(rules, policy.Rule{Action: rule.Action, Resource: rule.Resource, Effect: effect})
	}
	return rules
}
