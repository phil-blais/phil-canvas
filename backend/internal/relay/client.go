package relay

import (
	"time"

	"github.com/gorilla/websocket"

	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
)

const (
	// writeWait is the time allowed to write a single frame.
	writeWait = 10 * time.Second
	// pongWait is how long we wait for a pong before considering the peer dead.
	pongWait = 60 * time.Second
	// pingPeriod must be less than pongWait; we ping to keep proxies from
	// idling out the long-lived connection.
	pingPeriod = (pongWait * 9) / 10
	// maxMessageSize caps a single inbound frame. Generous because image files
	// are relayed peer-to-peer as (large) Yjs binary updates.
	maxMessageSize = 16 << 20 // 16 MiB
	// sendBuffer is the per-client outbound queue depth.
	sendBuffer = 256
)

// outgoing is a queued frame or a directive to close the connection.
type outgoing struct {
	kind   rooms.MessageKind
	data   []byte
	close  bool
	code   int
	reason string
}

// Client is one connected participant. It implements rooms.Client. All writes
// go through a single write pump goroutine, so the gorilla connection is never
// written concurrently.
type Client struct {
	conn *websocket.Conn
	room *rooms.Room
	send chan outgoing
	done chan struct{}
}

func newClient(conn *websocket.Conn, room *rooms.Room) *Client {
	return &Client{
		conn: conn,
		room: room,
		send: make(chan outgoing, sendBuffer),
		done: make(chan struct{}),
	}
}

// Send queues a frame. If the outbound buffer is full the client is too slow to
// keep up with the room, so we drop it rather than block the broadcaster.
func (c *Client) Send(kind rooms.MessageKind, msg []byte) {
	select {
	case c.send <- outgoing{kind: kind, data: msg}:
	case <-c.done:
	default:
		c.conn.Close() // safe to call concurrently; pumps will exit
	}
}

// CloseWithCode queues a close directive so the write pump emits a proper close
// frame (with the code) in order, after any already-queued messages. This is
// what lets the defensive close deliver room-closed then close with 4001.
func (c *Client) CloseWithCode(code int, reason string) {
	select {
	case c.send <- outgoing{close: true, code: code, reason: reason}:
	case <-c.done:
	default:
		c.conn.Close()
	}
}

// readPump reads frames and broadcasts them opaquely to the rest of the room.
// It owns teardown: on exit it deregisters the client and stops the write pump.
func (c *Client) readPump() {
	defer func() {
		c.room.RemoveClient(c)
		close(c.done)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var kind rooms.MessageKind
		switch messageType {
		case websocket.TextMessage:
			kind = rooms.TextMessage
		case websocket.BinaryMessage:
			kind = rooms.BinaryMessage
		default:
			continue
		}
		c.room.Broadcast(c, kind, data)
	}
}

// writePump serializes all writes to the connection: queued frames, periodic
// pings, and the final close frame.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if msg.close {
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(msg.code, msg.reason))
				return
			}
			if err := c.conn.WriteMessage(frameType(msg.kind), msg.data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

func frameType(kind rooms.MessageKind) int {
	if kind == rooms.TextMessage {
		return websocket.TextMessage
	}
	return websocket.BinaryMessage
}
