package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the server
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Security  SecurityConfig  `mapstructure:"security"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	Logging   LoggingConfig   `mapstructure:"logging"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Host             string `mapstructure:"host"`
	Port             int    `mapstructure:"port"`
	AdminPassword    string `mapstructure:"admin_password"`
	AgentEnrollToken string `mapstructure:"agent_enroll_token"`
}

// StorageConfig holds storage-specific configuration
type StorageConfig struct {
	ObjectsDir    string `mapstructure:"objects_dir"`
	DBPath        string `mapstructure:"db_path"`
	MaxFileSizeGB int    `mapstructure:"max_file_size_gb"`
}

// SecurityConfig holds security-specific configuration
type SecurityConfig struct {
	SessionTimeoutMinutes       int `mapstructure:"session_timeout_minutes"`
	DownloadTokenTimeoutMinutes int `mapstructure:"download_token_timeout_minutes"`
}

// WebSocketConfig holds WebSocket-specific configuration
type WebSocketConfig struct {
	PingIntervalSeconds      int `mapstructure:"ping_interval_seconds"`
	PongTimeoutSeconds       int `mapstructure:"pong_timeout_seconds"`
	HeartbeatIntervalSeconds int `mapstructure:"heartbeat_interval_seconds"`
}

// LoggingConfig holds logging-specific configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Output string `mapstructure:"output"`
}

// Addr returns the server address in host:port format
func (s *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// SessionTimeout returns the session timeout as a duration
func (s *SecurityConfig) SessionTimeout() time.Duration {
	return time.Duration(s.SessionTimeoutMinutes) * time.Minute
}

// DownloadTokenTimeout returns the download token timeout as a duration
func (s *SecurityConfig) DownloadTokenTimeout() time.Duration {
	return time.Duration(s.DownloadTokenTimeoutMinutes) * time.Minute
}

// PingInterval returns the ping interval as a duration
func (w *WebSocketConfig) PingInterval() time.Duration {
	return time.Duration(w.PingIntervalSeconds) * time.Second
}

// PongTimeout returns the pong timeout as a duration
func (w *WebSocketConfig) PongTimeout() time.Duration {
	return time.Duration(w.PongTimeoutSeconds) * time.Second
}

// HeartbeatInterval returns the heartbeat interval as a duration
func (w *WebSocketConfig) HeartbeatInterval() time.Duration {
	return time.Duration(w.HeartbeatIntervalSeconds) * time.Second
}

// Load loads configuration from file
func Load() (*Config, error) {
	return LoadFromFile("config.yaml")
}

// LoadFromFile loads configuration from a specific file
func LoadFromFile(filename string) (*Config, error) {
	viper.SetConfigFile(filename)
	viper.SetConfigType("yaml")

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 18120)
	viper.SetDefault("server.admin_password", "")
	viper.SetDefault("server.agent_enroll_token", "")

	// Storage defaults
	viper.SetDefault("storage.objects_dir", "./data/objects")
	viper.SetDefault("storage.db_path", "./data/meta.sqlite")
	viper.SetDefault("storage.max_file_size_gb", 10)

	// Security defaults
	viper.SetDefault("security.session_timeout_minutes", 60)
	viper.SetDefault("security.download_token_timeout_minutes", 10)

	// WebSocket defaults
	viper.SetDefault("websocket.ping_interval_seconds", 10)
	viper.SetDefault("websocket.pong_timeout_seconds", 30)
	viper.SetDefault("websocket.heartbeat_interval_seconds", 15)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.output", "stdout")
}

// validate validates the configuration
func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.Server.Port)
	}

	if cfg.Server.AdminPassword == "" {
		return fmt.Errorf("admin_password is required")
	}

	if cfg.Server.AgentEnrollToken == "" {
		return fmt.Errorf("agent_enroll_token is required")
	}

	if cfg.Storage.MaxFileSizeGB < 1 {
		return fmt.Errorf("max_file_size_gb must be at least 1")
	}

	return nil
}
