package relay

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/phil-blais/phil-canvas/backend/internal/rooms"
	"github.com/phil-blais/phil-canvas/backend/internal/token"
)

func newRelayServer(t *testing.T) (*rooms.Registry, *token.Issuer, string) {
	t.Helper()
	reg := rooms.NewRegistry()
	iss := token.NewIssuer([]byte("secret"), time.Hour)
	mux := http.NewServeMux()
	NewHandler(reg, iss).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return reg, iss, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func guestToken(t *testing.T, iss *token.Issuer, roomID string) string {
	t.Helper()
	tok, err := iss.IssueGuest("guest-"+roomID, roomID)
	if err != nil {
		t.Fatalf("issue guest token: %v", err)
	}
	return tok
}

// dial opens a raw connection without sending an auth frame.
func dial(t *testing.T, wsURL, roomID string) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL+"/ws/"+roomID, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// dialAuth opens a connection and sends the auth frame.
func dialAuth(t *testing.T, wsURL, roomID, tok string) *websocket.Conn {
	t.Helper()
	conn := dial(t, wsURL, roomID)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"auth","token":"`+tok+`"}`)); err != nil {
		t.Fatalf("write auth frame: %v", err)
	}
	return conn
}

// waitForCount blocks until the room reports n participants (or fails).
func waitForCount(t *testing.T, room *rooms.Room, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if room.ParticipantCount() == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for participant count %d (have %d)", n, room.ParticipantCount())
}

func expectCloseCode(t *testing.T, conn *websocket.Conn, code int) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("expected a close error, got %v", err)
	}
	if ce.Code != code {
		t.Fatalf("close code = %d, want %d", ce.Code, code)
	}
}

func TestRelayBroadcastsToOtherClients(t *testing.T) {
	reg, iss, wsURL := newRelayServer(t)
	room := reg.Create("owner", "", "")

	a := dialAuth(t, wsURL, room.ID, guestToken(t, iss, room.ID))
	b := dialAuth(t, wsURL, room.ID, guestToken(t, iss, room.ID))
	waitForCount(t, room, 2) // ensure both are registered before broadcasting

	payload := []byte{0x01, 0x02, 0x03} // opaque binary, as Yjs frames are
	if err := a.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatalf("client A write: %v", err)
	}

	_ = b.SetReadDeadline(time.Now().Add(2 * time.Second))
	mt, got, err := b.ReadMessage()
	if err != nil {
		t.Fatalf("client B read: %v", err)
	}
	if mt != websocket.BinaryMessage || string(got) != string(payload) {
		t.Errorf("B got (type=%d, %v), want binary %v", mt, got, payload)
	}

	// The sender must not receive its own message.
	_ = a.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, _, err := a.ReadMessage(); err == nil {
		t.Error("sender A should not receive its own broadcast")
	}
}

func TestRelayRejectsInvalidToken(t *testing.T) {
	reg, _, wsURL := newRelayServer(t)
	room := reg.Create("owner", "", "")

	conn := dialAuth(t, wsURL, room.ID, "not-a-valid-token")
	expectCloseCode(t, conn, closeCodeAuthFailed)
}

func TestRelayRejectsNonAuthFirstFrame(t *testing.T) {
	reg, _, wsURL := newRelayServer(t)
	room := reg.Create("owner", "", "")

	conn := dial(t, wsURL, room.ID)
	// First frame is Yjs binary instead of the required auth frame.
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{0x09}); err != nil {
		t.Fatalf("write: %v", err)
	}
	expectCloseCode(t, conn, closeCodeAuthFailed)
}

func TestRelayRejectsGuestForWrongRoom(t *testing.T) {
	reg, iss, wsURL := newRelayServer(t)
	room := reg.Create("owner", "", "")
	// Guest token bound to a different room must not grant access to this one.
	conn := dialAuth(t, wsURL, room.ID, guestToken(t, iss, "some-other-room"))
	expectCloseCode(t, conn, closeCodeAuthFailed)
}

func TestRelayUnknownRoomIs404(t *testing.T) {
	_, _, wsURL := newRelayServer(t)
	_, resp, err := websocket.DefaultDialer.Dial(wsURL+"/ws/does-not-exist", nil)
	if err == nil {
		t.Fatal("expected handshake to fail for unknown room")
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %v, want 404", resp)
	}
}

func TestRelayDefensiveCloseDeliversMessageThen4001(t *testing.T) {
	reg, iss, wsURL := newRelayServer(t)
	room := reg.Create("owner", "", "")

	conn := dialAuth(t, wsURL, room.ID, guestToken(t, iss, room.ID))
	waitForCount(t, room, 1)

	// Simulate DELETE /rooms/{id}'s relay side.
	room.CloseAll(rooms.RoomClosedMessage)

	// First: the room-closed text message.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	mt, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read room-closed message: %v", err)
	}
	if mt != websocket.TextMessage || string(data) != string(rooms.RoomClosedMessage) {
		t.Errorf("got (type=%d, %s), want text room-closed message", mt, data)
	}

	// Then: a close frame with code 4001.
	expectCloseCode(t, conn, rooms.CloseCodeRoomClosed)
}
