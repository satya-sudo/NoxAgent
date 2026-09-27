package api

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func (s *Server) browserKeepalive(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		!headerContains(r.Header.Get("Connection"), "upgrade") ||
		r.Header.Get("Sec-WebSocket-Version") != "13" {
		writeError(w, http.StatusBadRequest, "WebSocket upgrade required")
		return
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "Sec-WebSocket-Key is required")
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		writeError(w, http.StatusInternalServerError, "WebSocket upgrade is unavailable")
		return
	}
	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer connection.Close()
	if err := writeUpgrade(buffer, websocketAccept(key)); err != nil {
		return
	}
	s.broker.Touch()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			s.broker.Touch()
			if err := writeTextFrame(connection, "keepalive"); err != nil {
				return
			}
		}
	}
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func headerContains(value, expected string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), expected) {
			return true
		}
	}
	return false
}

func writeUpgrade(buffer *bufio.ReadWriter, accept string) error {
	if _, err := fmt.Fprintf(buffer, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept); err != nil {
		return err
	}
	return buffer.Flush()
}

func writeTextFrame(connection net.Conn, message string) error {
	if len(message) > 125 {
		return fmt.Errorf("keepalive frame is too large")
	}
	_ = connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
	frame := append([]byte{0x81, byte(len(message))}, []byte(message)...)
	_, err := connection.Write(frame)
	return err
}
