package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ziyi233/onebot-tui/adapter"
	"github.com/ziyi233/onebot-tui/config"
	"github.com/ziyi233/onebot-tui/storage"
	"github.com/ziyi233/onebot-tui/tui"
	"gopkg.in/yaml.v3"
)

type AppState struct {
	sync.RWMutex
	ActiveChatID string
	Bot          adapter.BotAdapter
	ChatTypes    map[string]string // a cache for chatID -> chatType ("group" or "private")
	ChatNames    map[string]string // a cache for chatID -> chat name
}

// GetChatType provides a thread-safe way to get the type of a chat.
func (s *AppState) GetChatType(chatID string) string {
	s.RLock()
	defer s.RUnlock()
	return s.ChatTypes[chatID]
}

// GetChatName returns the name of a chat by its ID.
func (s *AppState) GetChatName(chatID string) string {
	s.RLock()
	defer s.RUnlock()
	return s.ChatNames[chatID]
}

// corsMiddleware 是一个中间件，用于为所有响应添加 CORS 头
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 允许任何来源的请求，对于本地开发是安全的
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// 如果是 OPTIONS 预检请求，直接返回
		if r.Method == "OPTIONS" {
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	f, err := os.OpenFile("daemon.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	var cfg *config.Config
	if _, err := os.Stat("config.yml"); os.IsNotExist(err) {
		// If config file does not exist, run the interactive wizard
		cfg, err = createConfigWizard()
		if err != nil {
			log.Fatalf("Could not create configuration: %v", err)
		}
	} else {
		// If config file exists, load it
		cfg, err = config.LoadConfig("config.yml")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	store, err := storage.NewStore(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to init db: %v", err)
	}
	defer store.Close()

	bot := adapter.NewNapCatAdapter()
	if err := bot.Connect(cfg.WebSocketURL, cfg.AccessToken); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer bot.Disconnect()

	appState := &AppState{
		Bot:       bot,
		ChatTypes: make(map[string]string),
		ChatNames: make(map[string]string),
	}

	msgChan := make(chan adapter.Message)
	go bot.Listen(msgChan)
	tuiModel := tui.New(appState, bot, store, msgChan)
	p := tea.NewProgram(tuiModel, tea.WithAltScreen())

	// Populate caches in the background
	go populateCaches(bot, appState, p) // Non-blocking

	// Goroutine to listen for new messages from the bot and forward them to the TUI
	go func() {
		for msg := range msgChan {
			store.AddMessage(&msg) // Persist message to DB
			p.Send(msg)            // Send message to TUI for display
		}
	}()

	// Start the HTTP control server in a separate goroutine
	go startControlServer(appState, bot, p)

	log.Println("TUI is running. Press Ctrl+C to exit.")
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

// populateCaches fetches the initial friend and group lists from the bot adapter.
// It now runs in a goroutine and retries until it succeeds.
func populateCaches(bot adapter.BotAdapter, state *AppState, tuiProgram *tea.Program) {
	log.Println("Starting background cache population...")
	var friends []adapter.ChatInfo
	var groups []adapter.ChatInfo
	var err error

	for {
		friends, groups, err = bot.GetChats()
		if err == nil {
			break // Success
		}
		log.Printf("Failed to get chat lists: %v. Retrying in 10 seconds...", err)
		time.Sleep(10 * time.Second)
	}

	state.Lock()
	for _, friend := range friends {
		state.ChatTypes[friend.ID] = "private"
		state.ChatNames[friend.ID] = friend.Name
	}
	for _, group := range groups {
		state.ChatTypes[group.ID] = "group"
		state.ChatNames[group.ID] = group.Name
	}
	state.Unlock()

	log.Printf("Caches populated successfully with %d friends and %d groups.", len(friends), len(groups))

	// Notify the TUI that caches are ready
	(*tuiProgram).Send(tui.CachesPopulatedMsg{})
}

func createConfigWizard() (*config.Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("--- OneBot TUI Setup ---")
	fmt.Println("config.yml not found. Let's create one.")

	fmt.Print("Enter NapCat WebSocket URL (e.g., ws://127.0.0.1:3001): ")
	wsURL, _ := reader.ReadString('\n')
	wsURL = strings.TrimSpace(wsURL)

	fmt.Print("Enter Access Token (if any, otherwise press Enter): ")
	accessToken, _ := reader.ReadString('\n')
	accessToken = strings.TrimSpace(accessToken)

	cfg := &config.Config{
		WebSocketURL: wsURL,
		AccessToken:  accessToken,
		DatabasePath: "onebot.db", // Sensible default
	}

	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	err = os.WriteFile("config.yml", yamlData, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write config.yml: %w", err)
	}

	fmt.Println("config.yml created successfully.")
	return cfg, nil
}

func startControlServer(state *AppState, bot adapter.BotAdapter, p *tea.Program) {
	// 创建一个新的 http.ServeMux (路由)
	mux := http.NewServeMux()

	mux.HandleFunc("/set_active_chat", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		name := r.URL.Query().Get("name") // Get name from query
		if id == "" {
			http.Error(w, "missing chat id", http.StatusBadRequest)
			return
		}
		if name == "" { // If name is not provided, use the ID as a fallback
			name = id
		}

		state.Lock()
		state.ActiveChatID = id
		state.Unlock()

		// Send a message to the TUI to notify it of the change
		p.Send(tui.ActiveChatChangedMsg{ID: id, Name: name})

		fmt.Fprintf(w, "Active chat set to %s (%s)\n", id, name)
		log.Printf("Switched active chat to %s (%s)", id, name)
	})

	mux.HandleFunc("/send_message", func(w http.ResponseWriter, r *http.Request) {
		state.RLock()
		activeID := state.ActiveChatID
		if activeID == "" {
			state.RUnlock()
			http.Error(w, "No active chat set. Please set one via /set_active_chat", http.StatusBadRequest)
			return
		}
		chatType, ok := state.ChatTypes[activeID]
		state.RUnlock()

		if !ok {
			http.Error(w, fmt.Sprintf("Could not determine chat type for ID %s. Cache may be stale.", activeID), http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}

		bot.SendMessage(activeID, chatType, string(body))
		fmt.Fprintf(w, "Message sent to %s (%s)\n", activeID, chatType)
	})

	mux.HandleFunc("/get_chats", func(w http.ResponseWriter, r *http.Request) {
		friends, groups, err := bot.GetChats()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		state.Lock()
		// Clear the old cache and repopulate
		state.ChatTypes = make(map[string]string)
		state.ChatNames = make(map[string]string)
		for _, friend := range friends {
			state.ChatTypes[friend.ID] = "private"
			state.ChatNames[friend.ID] = friend.Name
		}
		for _, group := range groups {
			state.ChatTypes[group.ID] = "group"
			state.ChatNames[group.ID] = group.Name
		}
		state.Unlock()

		w.Header().Set("Content-Type", "application/json")
		allChats := append(groups, friends...)
		json.NewEncoder(w).Encode(allChats)
	})

	log.Println("Control server listening on :9090")
	// 将我们的 mux 包装在 CORS 中间件里
	if err := http.ListenAndServe("127.0.0.1:9090", corsMiddleware(mux)); err != nil {
		log.Fatalf("Control server failed: %v", err)
	}
}
