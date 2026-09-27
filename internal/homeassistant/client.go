package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var (
	ErrNotFound  = errors.New("Home Assistant entity not found")
	ErrAmbiguous = errors.New("Home Assistant entity name is ambiguous")
)

type Entity struct {
	ID         string         `json:"entity_id"`
	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes"`
}

func (e Entity) Name() string {
	if value, ok := e.Attributes["friendly_name"].(string); ok && value != "" {
		return value
	}
	parts := strings.SplitN(e.ID, ".", 2)
	return strings.ReplaceAll(parts[len(parts)-1], "_", " ")
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func New(baseURL, token string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Home Assistant URL")
	}
	if token == "" {
		return nil, fmt.Errorf("Home Assistant token is required")
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *Client) TurnOn(ctx context.Context, target string) (Entity, error) {
	return c.callEntityService(ctx, target, []string{"light", "switch"}, "turn_on")
}

func (c *Client) TurnOff(ctx context.Context, target string) (Entity, error) {
	return c.callEntityService(ctx, target, []string{"light", "switch"}, "turn_off")
}

func (c *Client) ActivateScene(ctx context.Context, target string) (Entity, error) {
	return c.callEntityService(ctx, target, []string{"scene"}, "turn_on")
}

func (c *Client) GetState(ctx context.Context, target string) (Entity, error) {
	entities, err := c.states(ctx)
	if err != nil {
		return Entity{}, err
	}
	return resolve(entities, target, nil)
}

func (c *Client) callEntityService(ctx context.Context, target string, domains []string, service string) (Entity, error) {
	entities, err := c.states(ctx)
	if err != nil {
		return Entity{}, err
	}
	entity, err := resolve(entities, target, domains)
	if err != nil {
		return Entity{}, err
	}
	domain := strings.SplitN(entity.ID, ".", 2)[0]
	body, _ := json.Marshal(map[string]string{"entity_id": entity.ID})
	if err := c.request(ctx, http.MethodPost, "/api/services/"+domain+"/"+service, body, nil); err != nil {
		return Entity{}, err
	}
	return entity, nil
}

func (c *Client) states(ctx context.Context) ([]Entity, error) {
	var entities []Entity
	if err := c.request(ctx, http.MethodGet, "/api/states", nil, &entities); err != nil {
		return nil, err
	}
	return entities, nil
}

func (c *Client) request(ctx context.Context, method, path string, body []byte, output any) error {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("Home Assistant request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("Home Assistant returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			return fmt.Errorf("decode Home Assistant response: %w", err)
		}
	}
	return nil
}

type candidate struct {
	entity Entity
	score  int
}

func resolve(entities []Entity, target string, domains []string) (Entity, error) {
	target = normalize(target)
	allowed := make(map[string]bool, len(domains))
	for _, domain := range domains {
		allowed[domain] = true
	}
	var candidates []candidate
	for _, entity := range entities {
		domain := strings.SplitN(entity.ID, ".", 2)[0]
		if len(allowed) > 0 && !allowed[domain] {
			continue
		}
		name := normalize(entity.Name())
		idName := normalize(strings.TrimPrefix(entity.ID, domain+"."))
		score := 0
		switch {
		case target == name || target == idName:
			score = 100
		case strings.Contains(name, target) || strings.Contains(idName, target):
			score = 60
		case strings.Contains(target, name) || strings.Contains(target, idName):
			score = 40
		}
		if score > 0 {
			candidates = append(candidates, candidate{entity: entity, score: score})
		}
	}
	if len(candidates) == 0 {
		return Entity{}, fmt.Errorf("%w: %s", ErrNotFound, target)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if len(candidates) > 1 && candidates[0].score == candidates[1].score {
		return Entity{}, fmt.Errorf("%w: %s", ErrAmbiguous, target)
	}
	return candidates[0].entity, nil
}

func normalize(value string) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer("_", " ", "-", " ").Replace(value)
	words := strings.Fields(value)
	filtered := words[:0]
	for _, word := range words {
		switch word {
		case "the", "light", "lights", "switch", "switches", "scene", "sensor":
			continue
		default:
			filtered = append(filtered, word)
		}
	}
	return strings.Join(filtered, " ")
}
