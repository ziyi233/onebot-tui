package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// OneBot v11 的 Action 结构
type onebotAction struct {
	Action string      `json:"action"`
	Params interface{} `json:"params,omitempty"`
	Echo   string      `json:"echo"`
}

// NapCatAdapter 实现了 BotAdapter 接口
type NapCatAdapter struct {
	conn             *websocket.Conn
	writeMutex       sync.Mutex
	responseChannels sync.Map
	echoCounter      int64
}

// --- 新增的 API 响应结构体 ---
type friendListResp struct {
	Data []struct {
		UserID   int64  `json:"user_id"`
		Nickname string `json:"nickname"`
	} `json:"data"`
}

type groupListResp struct {
	Data []struct {
		GroupID   int64  `json:"group_id"`
		GroupName string `json:"group_name"`
	} `json:"data"`
}

func NewNapCatAdapter() *NapCatAdapter { return &NapCatAdapter{} }

func (n *NapCatAdapter) Connect(wsURL string, accessToken string) error {
	header := http.Header{}
	if accessToken != "" {
		header.Set("Authorization", "Bearer "+accessToken)
	}
	var err error
	n.conn, _, err = websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("failed to dial websocket at %s: %w", wsURL, err)
	}
	log.Println("NapCat Adapter: Successfully connected.")
	return nil
}

func (n *NapCatAdapter) Disconnect() error {
	if n.conn != nil {
		return n.conn.Close()
	}
	return nil
}

func (n *NapCatAdapter) Listen(msgChan chan<- Message) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("FATAL: Panic in adapter.Listen goroutine: %v\n%s", r, debug.Stack())
			}
			close(msgChan)
		}()
		log.Println("Adapter listener goroutine started.")
		for {
			_, payload, err := n.conn.ReadMessage()
			if err != nil {
				log.Printf("Adapter ReadMessage error: %v. Listener is shutting down.", err)
				return
			}
			log.Printf("Adapter received raw payload: %s", string(payload))
			var raw map[string]interface{}
			if err := json.Unmarshal(payload, &raw); err != nil {
				log.Printf("Adapter failed to unmarshal raw json: %v", err)
				continue
			}
			if echo, ok := raw["echo"].(string); ok && echo != "" {
				if ch, loaded := n.responseChannels.Load(echo); loaded {
					ch.(chan []byte) <- payload
				}
			} else if postType, ok := raw["post_type"].(string); ok {
				switch postType {
				case "message":
					n.handleMessageEvent(raw, msgChan)
				}
			}
		}
	}()
}

func (n *NapCatAdapter) handleMessageEvent(raw map[string]interface{}, msgChan chan<- Message) {
	var msg Message
	msg.Time = time.Now()
	if message, ok := raw["raw_message"].(string); ok {
		msg.Content = message
	}
	if sender, ok := raw["sender"].(map[string]interface{}); ok {
		if nickname, ok := sender["nickname"].(string); ok {
			msg.SenderName = nickname
		}
	}
	if userID, ok := raw["user_id"].(float64); ok {
		msg.SenderID = strconv.FormatInt(int64(userID), 10)
	}
	if msgType, ok := raw["message_type"].(string); ok {
		msg.ChatType = msgType
		if msgType == "group" {
			if groupID, ok := raw["group_id"].(float64); ok {
				msg.ChatID = strconv.FormatInt(int64(groupID), 10)
			}
		} else {
			msg.ChatID = msg.SenderID
		}
	}
	if msg.ChatID != "" {
		msgChan <- msg
	}
}

func (n *NapCatAdapter) SendMessage(chatID string, chatType string, message string) error {
	action := onebotAction{Echo: "send_msg_" + chatID}
	if chatType == "group" {
		groupID, _ := strconv.ParseInt(chatID, 10, 64)
		action.Action = "send_group_msg"
		action.Params = struct {
			GroupID int64  `json:"group_id"`
			Message string `json:"message"`
		}{GroupID: groupID, Message: message}
	} else {
		userID, _ := strconv.ParseInt(chatID, 10, 64)
		action.Action = "send_private_msg"
		action.Params = struct {
			UserID  int64  `json:"user_id"`
			Message string `json:"message"`
		}{UserID: userID, Message: message}
	}
	return n.sendAction(action)
}

func (n *NapCatAdapter) GetChats() (friends []ChatInfo, groups []ChatInfo, err error) {
	var wg sync.WaitGroup
	var errs = make(chan error, 2)

	wg.Add(2)

	// 并发获取好友列表
	go func() {
		defer wg.Done()
		action := onebotAction{Action: "get_friend_list"}
		respPayload, e := n.sendRequest(action)
		if e != nil {
			errs <- e
			return
		}
		var resp friendListResp
		if e := json.Unmarshal(respPayload, &resp); e != nil {
			errs <- e
			return
		}
		for _, f := range resp.Data {
			friends = append(friends, ChatInfo{
				ID:   strconv.FormatInt(f.UserID, 10),
				Name: f.Nickname,
				Type: "private",
			})
		}
	}()

	// 并发获取群列表
	go func() {
		defer wg.Done()
		action := onebotAction{Action: "get_group_list"}
		respPayload, e := n.sendRequest(action)
		if e != nil {
			errs <- e
			return
		}
		var resp groupListResp
		if e := json.Unmarshal(respPayload, &resp); e != nil {
			errs <- e
			return
		}
		for _, g := range resp.Data {
			groups = append(groups, ChatInfo{
				ID:   strconv.FormatInt(g.GroupID, 10),
				Name: g.GroupName,
				Type: "group",
			})
		}
	}()

	wg.Wait()
	close(errs)

	// 检查是否有错误发生
	for e := range errs {
		if e != nil {
			return nil, nil, e // 返回遇到的第一个错误
		}
	}

	return friends, groups, nil
}

func (n *NapCatAdapter) sendAction(action onebotAction) error {
	n.writeMutex.Lock()
	defer n.writeMutex.Unlock()
	payload, err := json.Marshal(action)
	if err != nil {
		return err
	}
	return n.conn.WriteMessage(websocket.TextMessage, payload)
}

func (n *NapCatAdapter) sendRequest(action onebotAction) ([]byte, error) {
	echo := fmt.Sprintf("req_%d", atomic.AddInt64(&n.echoCounter, 1))
	action.Echo = echo
	respChan := make(chan []byte, 1)
	n.responseChannels.Store(echo, respChan)
	defer n.responseChannels.Delete(echo)
	if err := n.sendAction(action); err != nil {
		return nil, err
	}
	select {
	case respPayload := <-respChan:
		return respPayload, nil
	case <-time.After(10 * time.Second):
		return nil, errors.New("API request timed out for action: " + action.Action)
	}
}
