package wiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const port = 38899

type Device struct {
	Name       string `json:"name"`
	IP         string `json:"ip"`
	MAC        string `json:"mac,omitempty"`
	ModuleName string `json:"module_name,omitempty"`
}

type Settings struct {
	State       *bool `json:"state,omitempty"`
	Dimming     int   `json:"dimming,omitempty"`
	Temperature int   `json:"temp,omitempty"`
}

type Client struct {
	mu        sync.RWMutex
	devices   map[string]Device
	broadcast string
	timeout   time.Duration
}

func New(configured, broadcast string) (*Client, error) {
	if strings.TrimSpace(broadcast) == "" {
		broadcast = "255.255.255.255"
	}
	if net.ParseIP(broadcast) == nil {
		return nil, fmt.Errorf("invalid WiZ broadcast address %q", broadcast)
	}
	client := &Client{devices: make(map[string]Device), broadcast: broadcast, timeout: 1500 * time.Millisecond}
	for _, item := range strings.Split(configured, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, address, ok := strings.Cut(item, "=")
		name, address = strings.TrimSpace(name), strings.TrimSpace(address)
		if !ok || name == "" || net.ParseIP(address) == nil {
			return nil, fmt.Errorf("invalid NOX_WIZ_LIGHTS entry %q; use name=IPv4", item)
		}
		client.devices[normalize(name)] = Device{Name: name, IP: address}
	}
	return client, nil
}

func (c *Client) Devices() []Device {
	c.mu.RLock()
	defer c.mu.RUnlock()
	devices := make([]Device, 0, len(c.devices))
	for _, device := range c.devices {
		devices = append(devices, device)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices
}

func (c *Client) Discover(ctx context.Context) ([]Device, error) {
	listenConfig := net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var controlErr error
		if err := raw.Control(func(fd uintptr) {
			controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
		}); err != nil {
			return err
		}
		return controlErr
	}}
	packetConnection, err := listenConfig.ListenPacket(ctx, "udp4", "0.0.0.0:0")
	if err != nil {
		return nil, fmt.Errorf("open WiZ discovery socket: %w", err)
	}
	connection := packetConnection.(*net.UDPConn)
	defer connection.Close()
	if err := connection.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, err
	}
	request, _ := json.Marshal(map[string]any{
		"method": "registration",
		"params": map[string]any{"phoneMac": "AAAAAAAAAAAA", "register": false, "phoneIp": "1.2.3.4", "id": "1"},
	})
	target := &net.UDPAddr{IP: net.ParseIP(c.broadcast), Port: port}
	if _, err := connection.WriteToUDP(request, target); err != nil {
		return nil, fmt.Errorf("broadcast WiZ discovery: %w", err)
	}

	deadline := time.Now().Add(c.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetReadDeadline(deadline); err != nil {
		return nil, err
	}
	buffer := make([]byte, 8192)
	for {
		length, source, readErr := connection.ReadFromUDP(buffer)
		if readErr != nil {
			var networkError net.Error
			if errors.As(readErr, &networkError) && networkError.Timeout() {
				break
			}
			return nil, fmt.Errorf("read WiZ discovery response: %w", readErr)
		}
		var response struct {
			Result struct {
				MAC        string `json:"mac"`
				ModuleName string `json:"moduleName"`
			} `json:"result"`
			Params struct {
				MAC        string `json:"mac"`
				ModuleName string `json:"moduleName"`
			} `json:"params"`
		}
		if json.Unmarshal(buffer[:length], &response) != nil {
			continue
		}
		mac, module := response.Result.MAC, response.Result.ModuleName
		if mac == "" {
			mac, module = response.Params.MAC, response.Params.ModuleName
		}
		c.remember(source.IP.String(), mac, module)
	}
	return c.Devices(), nil
}

func (c *Client) Set(ctx context.Context, name string, settings Settings) (Device, error) {
	if err := validate(settings); err != nil {
		return Device{}, err
	}
	device, err := c.resolve(name)
	if err != nil && len(c.Devices()) == 0 {
		if _, discoveryErr := c.Discover(ctx); discoveryErr != nil {
			return Device{}, discoveryErr
		}
		device, err = c.resolve(name)
	}
	if err != nil {
		return Device{}, err
	}
	var response struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	if err := c.call(ctx, device.IP, "setPilot", settings, &response); err != nil {
		return Device{}, err
	}
	if response.Error != nil {
		return Device{}, fmt.Errorf("WiZ device rejected command: %v", response.Error)
	}
	return device, nil
}

func (c *Client) call(ctx context.Context, ip, method string, params any, target any) error {
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: c.timeout}
	connection, err := dialer.DialContext(ctx, "udp4", address)
	if err != nil {
		return fmt.Errorf("connect to WiZ device %s: %w", ip, err)
	}
	defer connection.Close()
	deadline := time.Now().Add(c.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"method": method, "params": params})
	if err != nil {
		return err
	}
	if _, err := connection.Write(payload); err != nil {
		return fmt.Errorf("send WiZ command: %w", err)
	}
	buffer := make([]byte, 8192)
	length, err := connection.Read(buffer)
	if err != nil {
		return fmt.Errorf("wait for WiZ response: %w", err)
	}
	if err := json.Unmarshal(buffer[:length], target); err != nil {
		return fmt.Errorf("decode WiZ response: %w", err)
	}
	return nil
}

func (c *Client) remember(ip, mac, module string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, device := range c.devices {
		if device.IP == ip {
			device.MAC, device.ModuleName = mac, module
			c.devices[key] = device
			return
		}
	}
	suffix := strings.ToLower(mac)
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	name := strings.Trim(strings.ToLower(module)+"-"+suffix, "-")
	if name == "" {
		name = "wiz-" + strings.ReplaceAll(ip, ".", "-")
	}
	c.devices[normalize(name)] = Device{Name: name, IP: ip, MAC: mac, ModuleName: module}
}

func (c *Client) resolve(name string) (Device, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	wanted := normalize(name)
	if device, ok := c.devices[wanted]; ok {
		return device, nil
	}
	var matches []Device
	for key, device := range c.devices {
		if strings.Contains(key, wanted) || strings.Contains(wanted, key) {
			matches = append(matches, device)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(c.devices) == 1 {
		for _, device := range c.devices {
			return device, nil
		}
	}
	if len(matches) > 1 {
		return Device{}, fmt.Errorf("WiZ device name %q is ambiguous", name)
	}
	return Device{}, fmt.Errorf("WiZ device %q was not found; run discovery or configure NOX_WIZ_LIGHTS", name)
}

func validate(settings Settings) error {
	if settings.State == nil && settings.Dimming == 0 && settings.Temperature == 0 {
		return errors.New("WiZ command has no settings")
	}
	if settings.Dimming != 0 && (settings.Dimming < 10 || settings.Dimming > 100) {
		return errors.New("WiZ brightness must be between 10 and 100 percent")
	}
	if settings.Temperature != 0 && (settings.Temperature < 2200 || settings.Temperature > 6500) {
		return errors.New("WiZ color temperature must be between 2200K and 6500K")
	}
	return nil
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "the ")
	value = strings.TrimPrefix(value, "wiz ")
	return strings.Join(strings.Fields(value), " ")
}
