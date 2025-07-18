package storage

import (
	"database/sql"
	"log"

	// 方案二：引入纯 Go 的驱动，它会将自己注册为 "sqlite"
	_ "modernc.org/sqlite"

	"github.com/ziyi233/onebot-tui/adapter"
)

// Store 结构体保持不变
type Store struct {
	db *sql.DB
}

// NewStore 创建并初始化一个新的 Store
func NewStore(dbPath string) (*Store, error) {
	// 关键：驱动名称是 "sqlite"，而不是 "sqlite3"
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	// 表结构定义保持不变
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS messages (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"chat_id" TEXT NOT NULL,
		"chat_type" TEXT NOT NULL,
		"sender_id" TEXT,
		"sender_name" TEXT,
		"content" TEXT,
		"timestamp" DATETIME
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	log.Println("Database initialized successfully with pure Go 'sqlite' driver.")
	return &Store{db: db}, nil
}

// Close, AddMessage, GetMessages 等其他所有函数都保持完全不变
// 因为它们都是通过标准的 database/sql 接口操作，不关心底层具体是哪个驱动

func (s *Store) Close() {
	s.db.Close()
}

func (s *Store) AddMessage(msg *adapter.Message) error {
	insertSQL := `INSERT INTO messages(chat_id, chat_type, sender_id, sender_name, content, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := s.db.Prepare(insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(msg.ChatID, msg.ChatType, msg.SenderID, msg.SenderName, msg.Content, msg.Time)
	return err
}

func (s *Store) GetMessages(chatID string, limit int) ([]adapter.Message, error) {
	querySQL := `SELECT chat_id, chat_type, sender_id, sender_name, content, timestamp FROM messages WHERE chat_id = ? ORDER BY timestamp DESC LIMIT ?`

	rows, err := s.db.Query(querySQL, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []adapter.Message
	for rows.Next() {
		var msg adapter.Message
		err := rows.Scan(&msg.ChatID, &msg.ChatType, &msg.SenderID, &msg.SenderName, &msg.Content, &msg.Time)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
    
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}