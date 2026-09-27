package gmail

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googlemail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var (
	ErrNotConnected = errors.New("Gmail is not connected")
	ErrInvalidState = errors.New("invalid or expired OAuth state")
)

type Config struct {
	CredentialsPath string
	TokenPath       string
	RedirectURL     string
}

type Status struct {
	Configured bool `json:"configured"`
	Connected  bool `json:"connected"`
}

type Message struct {
	ID       string `json:"id"`
	ThreadID string `json:"thread_id"`
	From     string `json:"from"`
	Subject  string `json:"subject"`
	Date     string `json:"date"`
	Snippet  string `json:"snippet"`
}

type Client struct {
	oauth     *oauth2.Config
	tokenPath string
	now       func() time.Time

	statesMu sync.Mutex
	states   map[string]time.Time
}

func New(settings Config) (*Client, error) {
	if strings.TrimSpace(settings.CredentialsPath) == "" {
		return nil, errors.New("Gmail credentials path is required")
	}
	if strings.TrimSpace(settings.TokenPath) == "" {
		return nil, errors.New("Gmail token path is required")
	}
	if strings.TrimSpace(settings.RedirectURL) == "" {
		return nil, errors.New("Gmail redirect URL is required")
	}
	credentials, err := os.ReadFile(settings.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("read Gmail OAuth credentials: %w", err)
	}
	oauthConfig, err := google.ConfigFromJSON(credentials, googlemail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parse Gmail OAuth credentials: %w", err)
	}
	oauthConfig.RedirectURL = settings.RedirectURL
	return &Client{
		oauth:     oauthConfig,
		tokenPath: settings.TokenPath,
		now:       time.Now,
		states:    make(map[string]time.Time),
	}, nil
}

func (c *Client) Status() Status {
	_, err := loadToken(c.tokenPath)
	return Status{Configured: true, Connected: err == nil}
}

func (c *Client) AuthorizationURL() (string, error) {
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("create OAuth state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	c.statesMu.Lock()
	now := c.now()
	for candidate, expires := range c.states {
		if !expires.After(now) {
			delete(c.states, candidate)
		}
	}
	c.states[state] = now.Add(10 * time.Minute)
	c.statesMu.Unlock()

	return c.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (c *Client) CompleteAuthorization(ctx context.Context, state, code string) error {
	if strings.TrimSpace(code) == "" {
		return errors.New("authorization code is required")
	}
	if !c.consumeState(state) {
		return ErrInvalidState
	}
	token, err := c.oauth.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange Gmail authorization code: %w", err)
	}
	if err := saveToken(c.tokenPath, token); err != nil {
		return err
	}
	return nil
}

func (c *Client) ListMessages(ctx context.Context, query string, limit int64) ([]Message, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	token, err := loadToken(c.tokenPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotConnected
		}
		return nil, err
	}
	httpClient := c.oauth.Client(ctx, token)
	service, err := googlemail.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create Gmail service: %w", err)
	}
	call := service.Users.Messages.List("me").MaxResults(limit)
	if strings.TrimSpace(query) != "" {
		call = call.Q(query)
	}
	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("list Gmail messages: %w", err)
	}

	messages := make([]Message, 0, len(result.Messages))
	for _, item := range result.Messages {
		message, getErr := service.Users.Messages.Get("me", item.Id).
			Format("metadata").
			MetadataHeaders("From", "Subject", "Date").
			Do()
		if getErr != nil {
			return nil, fmt.Errorf("get Gmail message metadata: %w", getErr)
		}
		summary := Message{ID: message.Id, ThreadID: message.ThreadId, Snippet: message.Snippet}
		if message.Payload != nil {
			for _, header := range message.Payload.Headers {
				switch strings.ToLower(header.Name) {
				case "from":
					summary.From = header.Value
				case "subject":
					summary.Subject = header.Value
				case "date":
					summary.Date = header.Value
				}
			}
		}
		messages = append(messages, summary)
	}
	return messages, nil
}

func (c *Client) consumeState(state string) bool {
	c.statesMu.Lock()
	defer c.statesMu.Unlock()
	expires, ok := c.states[state]
	delete(c.states, state)
	return ok && expires.After(c.now())
}

func loadToken(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode Gmail token: %w", err)
	}
	return &token, nil
}

func saveToken(path string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Gmail token directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open Gmail token file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("protect Gmail token file: %w", err)
	}
	if err := json.NewEncoder(file).Encode(token); err != nil {
		file.Close()
		return fmt.Errorf("write Gmail token: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Gmail token file: %w", err)
	}
	return nil
}
