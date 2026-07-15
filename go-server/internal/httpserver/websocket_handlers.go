package httpserver

import (
	"context"
	"net/http"
	"sync"
	"time"

	"aipi-go/internal/tasks"

	"github.com/gorilla/websocket"
)

var taskSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

func (r *Router) taskSocket(w http.ResponseWriter, req *http.Request) {
	conn, err := taskSocketUpgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	type subscribeMessage struct {
		Type   string `json:"type"`
		TaskID string `json:"taskId"`
	}
	var writeMu sync.Mutex
	subscriptions := map[string]chan tasks.Message{}
	defer func() {
		for taskID, ch := range subscriptions {
			r.taskHub.Unsubscribe(taskID, ch)
		}
	}()

	for {
		var message subscribeMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		if message.Type != "subscribe" || message.TaskID == "" {
			continue
		}
		if _, exists := subscriptions[message.TaskID]; exists {
			continue
		}
		ch := r.taskHub.Subscribe(message.TaskID)
		subscriptions[message.TaskID] = ch
		if message.TaskID != "all" {
			go r.sendCurrentTaskSnapshot(context.Background(), &writeMu, conn, message.TaskID)
		}
		go func(taskID string, events <-chan tasks.Message) {
			for event := range events {
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				writeMu.Lock()
				err := conn.WriteJSON(event)
				writeMu.Unlock()
				if err != nil {
					r.taskHub.Unsubscribe(taskID, ch)
					return
				}
			}
		}(message.TaskID, ch)
	}
}

func (r *Router) sendCurrentTaskSnapshot(ctx context.Context, writeMu *sync.Mutex, conn *websocket.Conn, taskID string) {
	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	task, err := tasks.NewRepository(r.db).FindByID(taskCtx, taskID)
	if err != nil || task == nil {
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	writeMu.Lock()
	_ = conn.WriteJSON(tasks.Message{Type: "task", Data: tasks.ToPublic(task)})
	writeMu.Unlock()
}
