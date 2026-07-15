package tasks

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

func (h *Hub) Subscribe(taskID string) chan Message {
	ch := make(chan Message, 16)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[taskID] == nil {
		h.subscribers[taskID] = map[chan Message]bool{}
	}
	h.subscribers[taskID][ch] = true
	return ch
}

func (h *Hub) Unsubscribe(taskID string, ch chan Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[taskID] == nil {
		return
	}
	if !h.subscribers[taskID][ch] {
		return
	}
	delete(h.subscribers[taskID], ch)
	close(ch)
	if len(h.subscribers[taskID]) == 0 {
		delete(h.subscribers, taskID)
	}
}

func (h *Hub) PublishTask(task Task) {
	h.publish(task.ID, Message{Type: "task", Data: ToPublic(&task)})
	h.publish("all", Message{Type: "task", Data: ToPublic(&task)})
}

func (h *Hub) PublishProgress(taskID string, progress any) {
	h.publish(taskID, Message{Type: "progress", Data: progress})
}

func (h *Hub) publish(taskID string, message Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers[taskID] {
		select {
		case ch <- message:
		default:
		}
	}
}
