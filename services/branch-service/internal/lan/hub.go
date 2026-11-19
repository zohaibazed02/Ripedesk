package lan

import (
	"sync"
)

type Event struct {
	SeqID       int64  `json:"seq_id"`
	EventType   string `json:"event_type"`
	AggregateID string `json:"aggregate_id"`
	PayloadJSON string `json:"payload_json"`
	CreatedAt   string `json:"created_at"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
	closed  bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan Event]struct{}),
	}
}

func (h *Hub) Subscribe() chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch
	}
	h.clients[ch] = struct{}{}
	return ch
}

func (h *Hub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}

func (h *Hub) Publish(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
			// Slow consumer will catch up via GET /lan/events?since_seq=
		}
	}
}

func (h *Hub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.clients {
		delete(h.clients, ch)
		close(ch)
	}
}
