package api

import (
	"axiomoffice/core/internal/config"
	featureflag "axiomoffice/core/internal/flag"
)

// capabilityRegistry: cac kha nang may doc duoc cua Core API v1. Them kha nang moi o DUNG mot
// cho nay (va cap nhat Protocol neu doi hop dong), de /health va client khong phai doan.
var capabilityRegistry = []string{
	"runs.idempotency",
	"approvals.saved",
}

// capabilities: ban do ten kha nang -> bat. Cac kha nang nen tang luon bat (khong doi mac dinh
// cua ban cu); co release-safe chi xuat hien khi dang bat, duoi ten flags.<name>.
func capabilities(cfg config.Config) map[string]bool {
	result := make(map[string]bool, len(capabilityRegistry))
	for _, name := range capabilityRegistry {
		result[name] = true
	}
	for _, name := range featureflag.Names(cfg) {
		result["flags."+name] = true
	}
	return result
}
