package users

import "sync"

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Message]bool
}

type Message struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func NewHub() *Hub {
	return &Hub{subscribers: map[string]map[chan Message]bool{}}
}

func (h *Hub) Subscribe(userID string) chan Message {
	ch := make(chan Message, 16)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[userID] == nil {
		h.subscribers[userID] = map[chan Message]bool{}
	}
	h.subscribers[userID][ch] = true
	return ch
}

func (h *Hub) Unsubscribe(userID string, ch chan Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[userID] == nil || !h.subscribers[userID][ch] {
		return
	}
	delete(h.subscribers[userID], ch)
	close(ch)
	if len(h.subscribers[userID]) == 0 {
		delete(h.subscribers, userID)
	}
}

func (h *Hub) PublishUser(user *User) {
	if user == nil {
		return
	}
	h.publish(user.ID, Message{Type: "user", Data: ToPublicUser(user)})
}

func (h *Hub) PublishUserData(userID string, data any) {
	if userID == "" {
		return
	}
	h.publish(userID, Message{Type: "user", Data: data})
}

func (h *Hub) publish(userID string, message Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers[userID] {
		select {
		case ch <- message:
		default:
		}
	}
}
