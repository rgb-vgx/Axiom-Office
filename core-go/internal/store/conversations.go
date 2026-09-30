package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

// ConversationRow: mot hoi thoai gan voi tai lieu (New_arch.md muc 8.5.9).
type ConversationRow struct {
	ID           string
	DocumentKey  *string
	DocumentName *string
	App          string
	Family       string
	Title        *string
	Summary      *string
	CreatedAt    string
	UpdatedAt    string
}

type MessageRow struct {
	ID        int64
	Seq       int
	Role      string
	Content   string
	CreatedAt string
}

// Conversations: hoi thoai va tin nhan (tin nhan user + cau tra loi cuoi + tom tat tool).
type Conversations struct{ db *DB }

func (d *DB) Conversations() *Conversations { return &Conversations{db: d} }

// Now: moc thoi gian giong ban .NET (yyyy-MM-ddTHH:mm:ss.fffZ).
func Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// NewID: "<prefix>" + 32 ky tu hex (giong Guid.ToString("N") cua ban .NET).
func NewID(prefix string) string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return prefix + "00000000000000000000000000000000"
	}
	return prefix + hex.EncodeToString(buffer)
}

// NewConversationID: id hoi thoai ("c_" + 32 ky tu hex).
func NewConversationID() string { return NewID("c_") }

func (c *Conversations) Create(app, family string, documentKey, documentName, title *string) string {
	id, now := NewConversationID(), Now()
	_, _ = c.db.sql.Exec(`INSERT INTO conversations (id, created_at, updated_at, app, family, document_key, document_name, title, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`, id, now, now, app, family, documentKey, documentName, title)
	return id
}

func (c *Conversations) AppendMessage(conversationID, role, content string) int {
	tx, err := c.db.sql.Begin()
	if err != nil {
		return 0
	}
	defer func() { _ = tx.Rollback() }()

	var seq int
	_ = tx.QueryRow("SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE conversation_id = ?", conversationID).Scan(&seq)
	_, _ = tx.Exec(`INSERT INTO messages (conversation_id, seq, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		conversationID, seq, role, content, Now())
	_, _ = tx.Exec("UPDATE conversations SET updated_at = ? WHERE id = ?", Now(), conversationID)
	if err := tx.Commit(); err != nil {
		return 0
	}
	return seq
}

// Messages: limit > 0 -> lay cac tin MOI NHAT roi dao lai de doc theo thu tu thoi gian.
func (c *Conversations) Messages(conversationID string, limit int) []MessageRow {
	query := "SELECT id, seq, role, content, created_at FROM messages WHERE conversation_id = ? ORDER BY seq"
	args := []any{conversationID}
	if limit > 0 {
		query = "SELECT id, seq, role, content, created_at FROM (SELECT * FROM messages WHERE conversation_id = ? ORDER BY seq DESC LIMIT ?) ORDER BY seq"
		args = append(args, limit)
	}
	rows := []MessageRow{}
	result, err := c.db.sql.Query(query, args...)
	if err != nil {
		return rows
	}
	defer result.Close()
	for result.Next() {
		var message MessageRow
		if result.Scan(&message.ID, &message.Seq, &message.Role, &message.Content, &message.CreatedAt) == nil {
			rows = append(rows, message)
		}
	}
	if result.Err() != nil {
		return []MessageRow{}
	}
	return rows
}

func (c *Conversations) Get(id string) *ConversationRow {
	row := c.db.sql.QueryRow(`SELECT id, document_key, document_name, app, family, title, summary, created_at, updated_at
		FROM conversations WHERE id = ?`, id)
	return scanConversation(row)
}

func (c *Conversations) List(documentKey *string, limit int) []ConversationRow {
	query := `SELECT id, document_key, document_name, app, family, title, summary, created_at, updated_at
		FROM conversations ORDER BY updated_at DESC LIMIT ?`
	args := []any{limit}
	if documentKey != nil {
		query = `SELECT id, document_key, document_name, app, family, title, summary, created_at, updated_at
			FROM conversations WHERE document_key = ? ORDER BY updated_at DESC LIMIT ?`
		args = []any{*documentKey, limit}
	}
	rows := []ConversationRow{}
	result, err := c.db.sql.Query(query, args...)
	if err != nil {
		return rows
	}
	defer result.Close()
	for result.Next() {
		if row := scanConversation(result); row != nil {
			rows = append(rows, *row)
		}
	}
	if result.Err() != nil {
		return []ConversationRow{}
	}
	return rows
}

func (c *Conversations) Rename(id string, title, summary *string) bool {
	result, err := c.db.sql.Exec(`UPDATE conversations
		SET title = COALESCE(?, title), summary = COALESCE(?, summary), updated_at = ? WHERE id = ?`,
		title, summary, Now(), id)
	if err != nil {
		return false
	}
	affected, _ := result.RowsAffected()
	return affected > 0
}

func (c *Conversations) Delete(id string) bool {
	result, err := c.db.sql.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return false
	}
	affected, _ := result.RowsAffected()
	return affected > 0
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanConversation(row rowScanner) *ConversationRow {
	var (
		conversation          ConversationRow
		key, name, title, sum sql.NullString
	)
	if row.Scan(&conversation.ID, &key, &name, &conversation.App, &conversation.Family, &title, &sum,
		&conversation.CreatedAt, &conversation.UpdatedAt) != nil {
		return nil
	}
	conversation.DocumentKey = nullString(key)
	conversation.DocumentName = nullString(name)
	conversation.Title = nullString(title)
	conversation.Summary = nullString(sum)
	return &conversation
}

func nullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}
