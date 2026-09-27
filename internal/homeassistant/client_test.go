package homeassistant

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestTurnOnResolvesFriendlyNameAndCallsService(t *testing.T) {
	client, err := New("http://homeassistant.local:8123", "secret")
	if err != nil {
		t.Fatal(err)
	}
	var servicePath string
	client.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("authorization header was not set")
		}
		if request.URL.Path == "/api/states" {
			return response(200, `[
                  {"entity_id":"light.living_room","state":"off","attributes":{"friendly_name":"Living Room Lights"}},
                  {"entity_id":"switch.coffee","state":"off","attributes":{"friendly_name":"Coffee Machine"}}
                ]`), nil
		}
		servicePath = request.URL.Path
		return response(200, `[]`), nil
	})

	entity, err := client.TurnOn(context.Background(), "the living room lights")
	if err != nil {
		t.Fatal(err)
	}
	if entity.ID != "light.living_room" || servicePath != "/api/services/light/turn_on" {
		t.Fatalf("entity = %#v, service path = %s", entity, servicePath)
	}
}

func TestResolveRejectsAmbiguousName(t *testing.T) {
	entities := []Entity{
		{ID: "light.bedroom_left", Attributes: map[string]any{"friendly_name": "Bedroom Left Light"}},
		{ID: "light.bedroom_right", Attributes: map[string]any{"friendly_name": "Bedroom Right Light"}},
	}
	_, err := resolve(entities, "bedroom", []string{"light"})
	if err == nil || !strings.Contains(err.Error(), ErrAmbiguous.Error()) {
		t.Fatalf("resolve() error = %v, want ambiguous", err)
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
