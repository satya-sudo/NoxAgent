package api

import "testing"

func TestWebsocketAccept(t *testing.T) {
	got := websocketAccept("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("websocketAccept() = %q, want %q", got, want)
	}
}

func TestHeaderContains(t *testing.T) {
	if !headerContains("keep-alive, Upgrade", "upgrade") {
		t.Fatal("headerContains did not find a case-insensitive token")
	}
}
