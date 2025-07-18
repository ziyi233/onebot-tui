package adapter

import "time"

// Message 代表一条聊天消息，是程序内部流转的通用结构
type Message struct {
	ChatID     string    // 群号或 QQ 号
	ChatType   string    // "group" 或 "private"
	SenderID   string    // 发送者 QQ 号
	SenderName string    // 发-送者昵称
	Content    string    // 消息内容
	Time       time.Time // 消息时间
}

// ChatInfo 代表一个聊天会话（私聊或群聊），用于在 TUI 左侧列表显示
type ChatInfo struct {
	ID        string // 群号或 QQ 号
	Name      string // 群名称或好友昵称
	Type      string // "group" 或 "private"
	LatestMsg string // 最新一条消息预览
}

// BotAdapter 定义了所有后端适配器都必须实现的通用方法
type BotAdapter interface {
	// Connect 连接到 OneBot 后端
	Connect(wsURL string, accessToken string) error
	// Disconnect 断开连接
	Disconnect() error

	// SendMessage 发送消息到指定聊天
	SendMessage(chatID string, chatType string, message string) error

	// GetChats 获取分离的好友和群聊列表
	GetChats() (friends []ChatInfo, groups []ChatInfo, err error)

	// Listen 开始监听并接收一个 channel，它的职责是把从 WebSocket 收到的消息送入这个 channel
	Listen(msgChan chan<- Message)
}
