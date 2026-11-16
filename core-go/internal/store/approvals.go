package store

import (
	"database/sql"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
)

// Scope cua mot phe duyet da luu (schema v5, POST /v1/approvals).
const (
	ApprovalScopeDocument = "document"
	ApprovalScopeGlobal   = "global"
)

// ApprovalRow: mot phe duyet cho lenh rui ro - luot sau gap action+resource nay thi khong hoi lai
// nguoi dung (agent/confirm.go). ExpiresAt rong = khong het han.
type ApprovalRow struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	Scope     string `json:"scope"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}

// Approvals: phe duyet da luu cho lenh rui ro.
type Approvals struct{ db *DB }

func (d *DB) Approvals() *Approvals { return &Approvals{db: d} }

// AddApproval: luu mot phe duyet. resource rong = "*" (moi tai lieu), scope la khac document/global thi
// coi nhu document. Loi ghi tra ve cho ben goi (POST /v1/approvals phai biet de bao loi).
func (a *Approvals) AddApproval(action, resource, scope, expiresAt string) (ApprovalRow, error) {
	action = strings.TrimSpace(action)
	resource = strings.TrimSpace(resource)
	if scope != ApprovalScopeGlobal && scope != ApprovalScopeDocument {
		scope = ApprovalScopeDocument
	}
	if resource == "" {
		resource = "*"
	}
	row := ApprovalRow{
		ID:        NewID("ap_"),
		Action:    action,
		Resource:  resource,
		Scope:     scope,
		ExpiresAt: strings.TrimSpace(expiresAt),
		CreatedAt: Now(),
	}
	_, err := a.db.sql.Exec(`INSERT INTO permission_approvals (id, action, resource, scope, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, row.ID, row.Action, row.Resource, row.Scope, row.ExpiresAt, row.CreatedAt)
	if err != nil {
		return ApprovalRow{}, err
	}
	return row, nil
}

// ListApprovals: moi phe duyet, moi nhat truoc (GET /v1/approvals). Loi doc -> danh sach rong.
func (a *Approvals) ListApprovals() []ApprovalRow {
	rows := []ApprovalRow{}
	result, err := a.db.sql.Query(`SELECT id, action, resource, scope, expires_at, created_at
		FROM permission_approvals ORDER BY created_at DESC, id DESC`)
	if err != nil {
		corelog.Error("core.db: list approvals failed: %v", err)
		return rows
	}
	defer func() { _ = result.Close() }()
	for result.Next() {
		var row ApprovalRow
		var expires, created sql.NullString
		if result.Scan(&row.ID, &row.Action, &row.Resource, &row.Scope, &expires, &created) != nil {
			continue
		}
		row.ExpiresAt = expires.String
		row.CreatedAt = created.String
		rows = append(rows, row)
	}
	return rows
}

// DeleteApproval: xoa mot phe duyet; false = khong co id do.
func (a *Approvals) DeleteApproval(id string) bool {
	result, err := a.db.sql.Exec("DELETE FROM permission_approvals WHERE id = ?", strings.TrimSpace(id))
	if err != nil {
		corelog.Error("core.db: delete approval %s failed: %v", id, err)
		return false
	}
	count, err := result.RowsAffected()
	return err == nil && count > 0
}

// ActiveApproval: phe duyet con hieu luc cho action+resource. Action so khong phan biet hoa/thuong;
// resource trung (bo qua '/' va '\', khong phan biet hoa/thuong) hoac scope = global; expires_at rong =
// khong het han, con lai phai con han. Nhieu phe duyet khop -> cai moi nhat.
func (a *Approvals) ActiveApproval(action, resource string) *ApprovalRow {
	action = strings.TrimSpace(action)
	if action == "" {
		return nil
	}
	rows, err := a.db.sql.Query(`SELECT id, action, resource, scope, expires_at, created_at
		FROM permission_approvals WHERE action = ? COLLATE NOCASE
		ORDER BY created_at DESC, id DESC`, action)
	if err != nil {
		corelog.Error("core.db: active approval for %s failed: %v", action, err)
		return nil
	}
	defer func() { _ = rows.Close() }()

	wanted := normalizeResource(resource)
	now := time.Now().UTC()
	for rows.Next() {
		var row ApprovalRow
		var expires, created sql.NullString
		if rows.Scan(&row.ID, &row.Action, &row.Resource, &row.Scope, &expires, &created) != nil {
			continue
		}
		row.ExpiresAt = expires.String
		row.CreatedAt = created.String
		if row.Scope != ApprovalScopeGlobal && normalizeResource(row.Resource) != wanted {
			continue
		}
		if approvalExpired(row.ExpiresAt, now) {
			continue
		}
		found := row
		return &found
	}
	return nil
}

// approvalExpired: expires_at rong = khong het han; gia tri khong doc duoc coi nhu het han (an toan hon).
func approvalExpired(expiresAt string, now time.Time) bool {
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt == "" {
		return false
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true
	}
	return !now.Before(expires)
}

func normalizeResource(resource string) string {
	value := strings.ReplaceAll(strings.TrimSpace(resource), `\`, "/")
	return strings.ToLower(strings.TrimRight(value, "/"))
}
