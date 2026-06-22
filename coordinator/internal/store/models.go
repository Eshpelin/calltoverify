package store

import "time"

// AppConfig is the per-app configuration stored as JSONB on apps.config.
type AppConfig struct {
	BindingMode     string   `json:"binding_mode,omitempty"`
	CodeLen         int      `json:"code_len,omitempty"`
	TTLSeconds      int      `json:"ttl_seconds,omitempty"`
	ChannelsEnabled []string `json:"channels_enabled,omitempty"`
}

// App is a tenant / API key.
type App struct {
	ID            string
	Name          string
	APIKeyHash    string
	APIKeyPrefix  string
	WebhookURL    string
	WebhookSecret string
	Config        AppConfig
	CreatedAt     time.Time
}

// Device is a receiver: a spare Android phone or a Raspberry Pi + GSM modem.
type Device struct {
	ID            string
	AppID         string
	Name          string
	DeviceSecret  string
	Type          string
	Capabilities  []string
	Status        string
	LastHeartbeat *time.Time
	CreatedAt     time.Time
}

// Number is an MSISDN hosted by a device.
type Number struct {
	ID        string
	DeviceID  string
	MSISDN    string
	Channels  []string
	Status    string
	CreatedAt time.Time
}

// Session is a verification session.
type Session struct {
	ID             string
	AppID          string
	Channel        string
	BindingMode    string
	Status         string
	NumberID       *string
	Code           *string
	ClaimedMSISDN  *string
	VerifiedMSISDN *string
	Attempts       int
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// InboundEvent is a raw inbound signal reported by a receiver.
type InboundEvent struct {
	NumberID         string
	Type             string // sms | call
	Sender           string
	Body             string
	MatchedSessionID *string
}
