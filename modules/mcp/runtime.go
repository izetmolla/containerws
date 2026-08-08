package mcp

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// StandaloneStatus is the live + desired state of the dedicated MCP listener.
type StandaloneStatus struct {
	Enabled       bool                `json:"enabled"`
	Running       bool                `json:"running"`
	Address       string              `json:"address"`
	Port          int                 `json:"port"`
	ListenAddr    string              `json:"listen_addr"`
	PublicURL     string              `json:"public_url"`
	LastError     string              `json:"last_error,omitempty"`
	Source        string              `json:"source"` // "settings" | "env" | "off"
	MainAPIMCP    string              `json:"main_api_mcp"`
	BindAddresses []BindAddressOption `json:"bind_addresses"`
}

type standaloneRuntime struct {
	mu      sync.Mutex
	app     *fiber.App
	addr    string
	lastErr string
}

var runtime = &standaloneRuntime{}

// StandaloneRunning reports whether the dedicated MCP Fiber app is active.
func StandaloneRunning() bool {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.app != nil
}

// CurrentStandaloneAddr returns the listen address of the running standalone MCP (or empty).
func CurrentStandaloneAddr() string {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.addr
}

func (r *standaloneRuntime) setRunning(app *fiber.App, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.app = app
	r.addr = addr
	r.lastErr = ""
}

func (r *standaloneRuntime) setError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		r.lastErr = ""
		return
	}
	r.lastErr = err.Error()
}

func (r *standaloneRuntime) clear() *fiber.App {
	r.mu.Lock()
	defer r.mu.Unlock()
	app := r.app
	r.app = nil
	r.addr = ""
	return app
}

func (r *standaloneRuntime) lastError() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr
}

// LoadStandaloneConfig reads desired standalone settings from options, falling back to env.
func LoadStandaloneConfig(db *gorm.DB) (enabled bool, address string, port int, source string) {
	address = "0.0.0.0"
	port = 0
	source = "off"

	if db != nil {
		if on, ok, err := models.GetOptionBool(db, models.OptionMCPStandaloneEnabled); err == nil && ok {
			enabled = on
			source = "settings"
		}
		if v, ok, err := models.GetOption(db, models.OptionMCPStandaloneAddress); err == nil && ok && strings.TrimSpace(v) != "" {
			address = strings.TrimSpace(v)
			source = "settings"
		}
		if v, ok, err := models.GetOption(db, models.OptionMCPStandalonePort); err == nil && ok && strings.TrimSpace(v) != "" {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
				port = n
				source = "settings"
			}
		}
	}

	// Env fills gaps / used when settings never configured.
	if port <= 0 {
		envPort := strings.TrimSpace(viper.GetString("MCP_PORT"))
		if standalonePortConfigured(envPort) {
			if n, err := strconv.Atoi(envPort); err == nil {
				port = n
				enabled = true
				if source == "off" {
					source = "env"
				}
			}
		}
	}
	if address == "" || (source == "env" && strings.TrimSpace(viper.GetString("MCP_ADDRESS")) != "") {
		if h := strings.TrimSpace(viper.GetString("MCP_ADDRESS")); h != "" {
			address = h
		}
	}
	if address == "" {
		address = "0.0.0.0"
	}
	if !enabled && source == "settings" {
		port = max(port, 0)
	}
	return enabled, address, port, source
}

// GetStandaloneStatus builds a status payload for the settings UI.
func GetStandaloneStatus(appClients *config.AppClients) StandaloneStatus {
	db := (*gorm.DB)(nil)
	if appClients != nil {
		db = appClients.DB()
	}
	enabled, address, port, source := LoadStandaloneConfig(db)
	running := StandaloneRunning()
	addr := CurrentStandaloneAddr()
	if addr == "" && port > 0 {
		addr = net.JoinHostPort(address, strconv.Itoa(port))
	}

	st := StandaloneStatus{
		Enabled:       enabled,
		Running:       running,
		Address:       address,
		Port:          port,
		ListenAddr:    addr,
		PublicURL:     "",
		LastError:     runtime.lastError(),
		Source:        source,
		MainAPIMCP:    mainAPIMCPURL(appClients),
		BindAddresses: EnsureBindAddressOption(ListBindAddresses(), address),
	}
	if running || (enabled && port > 0) {
		st.PublicURL = standalonePublicURL(appClients, address, port)
	}
	return st
}

// ApplyStandaloneConfig persists options and starts/stops the listener to match.
func ApplyStandaloneConfig(appClients *config.AppClients, enabled bool, address string, port int) (StandaloneStatus, error) {
	if appClients == nil || appClients.DB() == nil {
		return StandaloneStatus{}, fmt.Errorf("database unavailable")
	}
	address = strings.TrimSpace(address)
	if address == "" {
		address = "0.0.0.0"
	}
	if enabled {
		if port <= 0 || port > 65535 {
			return StandaloneStatus{}, fmt.Errorf("port must be between 1 and 65535")
		}
		if _, err := net.LookupPort("tcp", strconv.Itoa(port)); err != nil {
			return StandaloneStatus{}, fmt.Errorf("invalid port: %w", err)
		}
	}

	db := appClients.DB()
	if err := models.SetOptionBool(db, models.OptionMCPStandaloneEnabled, enabled); err != nil {
		return StandaloneStatus{}, err
	}
	if err := models.SetOption(db, models.OptionMCPStandaloneAddress, address); err != nil {
		return StandaloneStatus{}, err
	}
	if err := models.SetOption(db, models.OptionMCPStandalonePort, strconv.Itoa(port)); err != nil {
		return StandaloneStatus{}, err
	}

	if !enabled {
		if err := StopStandalone(); err != nil {
			runtime.setError(err)
			return GetStandaloneStatus(appClients), err
		}
		return GetStandaloneStatus(appClients), nil
	}

	if err := RestartStandalone(appClients, address, port); err != nil {
		runtime.setError(err)
		_ = models.SetOptionBool(db, models.OptionMCPStandaloneEnabled, false)
		return GetStandaloneStatus(appClients), err
	}
	return GetStandaloneStatus(appClients), nil
}

// StopStandalone shuts down the dedicated MCP listener if running.
func StopStandalone() error {
	app := runtime.clear()
	if app == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("MCP standalone stop: %v", err)
		return err
	}
	log.Printf("MCP standalone stopped")
	return nil
}

// RestartStandalone stops any existing listener and starts on address:port.
func RestartStandalone(appClients *config.AppClients, address string, port int) error {
	_ = StopStandalone()
	address = strings.TrimSpace(address)
	if address == "" {
		address = "0.0.0.0"
	}
	addr := net.JoinHostPort(address, strconv.Itoa(port))
	token := strings.TrimSpace(viper.GetString("MCP_TOKEN"))
	app, err := newStandaloneApp(appClients, token)
	if err != nil {
		return err
	}
	runtime.setRunning(app, addr)
	go func() {
		log.Printf("MCP standalone listening on http://%s", normalizeListenLogAddr(addr))
		if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			log.Printf("MCP standalone stopped: %v", err)
			runtime.mu.Lock()
			if runtime.app == app {
				runtime.app = nil
				runtime.addr = ""
				runtime.lastErr = err.Error()
			}
			runtime.mu.Unlock()
		}
	}()
	// Brief settle — fail fast if bind is rejected.
	time.Sleep(150 * time.Millisecond)
	if errMsg := runtime.lastError(); errMsg != "" && !StandaloneRunning() {
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

// StartStandaloneFromSettingsOrEnv boots MCP from DB options when enabled,
// otherwise falls back to MCP_PORT env (legacy). Registers the app in runtime.
func StartStandaloneFromSettingsOrEnv(appClients *config.AppClients) (*fiber.App, error) {
	var db *gorm.DB
	if appClients != nil {
		db = appClients.DB()
	}
	enabled, address, port, source := LoadStandaloneConfig(db)
	if !enabled || port <= 0 {
		return nil, nil
	}
	log.Printf("MCP standalone: starting from %s on %s:%d", source, address, port)
	if err := RestartStandalone(appClients, address, port); err != nil {
		return nil, err
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.app, nil
}

// RegisterExternalStandalone records an app started outside ApplyStandaloneConfig
// (kept for compatibility; prefer StartStandaloneFromSettingsOrEnv).
func RegisterExternalStandalone(app *fiber.App, addr string) {
	if app == nil {
		return
	}
	runtime.setRunning(app, addr)
}
