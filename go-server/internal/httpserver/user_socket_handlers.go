package httpserver

import (
	"context"
	"net/http"
	"sync"
	"time"

	"aipi-go/internal/users"

	"github.com/gorilla/websocket"
)

var userSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

func (r *Router) userSocket(w http.ResponseWriter, req *http.Request) {
	conn, err := userSocketUpgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	type subscribeMessage struct {
		Type   string `json:"type"`
		UserID string `json:"userId"`
	}
	var writeMu sync.Mutex
	subscriptions := map[string]chan users.Message{}
	defer func() {
		for userID, ch := range subscriptions {
			r.userHub.Unsubscribe(userID, ch)
		}
	}()

	for {
		var message subscribeMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		if message.Type != "subscribe" || message.UserID == "" {
			continue
		}
		if _, exists := subscriptions[message.UserID]; exists {
			continue
		}
		ch := r.userHub.Subscribe(message.UserID)
		subscriptions[message.UserID] = ch
		go r.sendCurrentUserSnapshot(req.Context(), &writeMu, conn, message.UserID)
		go func(userID string, events <-chan users.Message) {
			for event := range events {
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				writeMu.Lock()
				err := conn.WriteJSON(event)
				writeMu.Unlock()
				if err != nil {
					r.userHub.Unsubscribe(userID, ch)
					return
				}
			}
		}(message.UserID, ch)
	}
}

func (r *Router) sendCurrentUserSnapshot(ctx context.Context, writeMu *sync.Mutex, conn *websocket.Conn, userID string) {
	userCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(userCtx, userID)
	if err != nil || user == nil {
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	writeMu.Lock()
	_ = conn.WriteJSON(users.Message{Type: "user", Data: r.publicUserWithSubscription(userCtx, user)})
	writeMu.Unlock()
}
