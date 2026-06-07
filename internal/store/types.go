package store

import "database/sql"

// Device is a registered TRMNL device.
type Device struct {
	ID              int64
	MAC             string
	APIKey          string
	FriendlyID      string
	Name            sql.NullString
	Model           sql.NullString
	FWVersion       sql.NullString
	Width           int
	Height          int
	RefreshRate     int
	PlaylistID      sql.NullInt64
	PlaylistCursor  int
	BatteryVoltage  sql.NullFloat64
	BatteryCharging sql.NullBool
	RSSI            sql.NullInt64
	WifiStatus      sql.NullString
	LastSeenAt      sql.NullInt64
	CreatedAt       int64
	// Firmware/special-function controls surfaced in /api/display.
	FirmwareUpdate  bool
	FirmwareURL     sql.NullString
	ResetFirmware   bool
	SpecialFunction sql.NullString
}

// DeviceLog is a single log entry reported by a device.
type DeviceLog struct {
	ID              int64
	DeviceID        int64
	LogID           sql.NullInt64
	Message         sql.NullString
	CreatedAt       sql.NullInt64
	ReceivedAt      int64
	WifiStatus      sql.NullString
	WifiSignal      sql.NullInt64
	SleepDuration   sql.NullInt64
	RefreshRate     sql.NullInt64
	FreeHeapSize    sql.NullInt64
	MaxAllocSize    sql.NullInt64
	SourcePath      sql.NullString
	SourceLine      sql.NullInt64
	WakeReason      sql.NullString
	FirmwareVersion sql.NullString
	BatteryVoltage  sql.NullFloat64
	SpecialFunction sql.NullString
}

// Plugin is a configured plugin instance.
type Plugin struct {
	ID        int64
	Type      string
	Name      string
	CreatedAt int64
}

// Screen binds a plugin instance + settings into a renderable.
type Screen struct {
	ID           int64
	PluginID     int64
	Name         string
	SettingsJSON string
	DitherMode   sql.NullString // NULL == inherit the global dither_mode setting
	RenderedHash sql.NullString
	RenderedAt   sql.NullInt64
	CreatedAt    int64
}

// Playlist is an ordered collection of screens.
type Playlist struct {
	ID        int64
	Name      string
	CreatedAt int64
}

// PlaylistItem places a screen at a position within a playlist.
type PlaylistItem struct {
	ID         int64
	PlaylistID int64
	ScreenID   int64
	Position   int
	Visible    bool
}

// CalendarAccount is a configured calendar source (a single Google or CalDAV
// account). Credentials and provider-specific options live in ConfigJSON.
type CalendarAccount struct {
	ID              int64
	Provider        string
	Name            string
	Marker          string
	ConfigJSON      string
	RefreshInterval int // seconds between syncs
	LastSync        sql.NullInt64
	LastError       sql.NullString
	CreatedAt       int64
}

// OAuthClient is a stored OAuth application credential (e.g. a Google Cloud
// OAuth client) that calendar accounts authorize against. ClientSecret is
// encrypted at rest when a secret key is configured.
type OAuthClient struct {
	ID           int64
	Provider     string
	Name         string
	ClientID     string
	ClientSecret string
	CreatedAt    int64
}

// CalendarEvent is one cached event instance belonging to an account.
type CalendarEvent struct {
	ID        int64
	AccountID int64
	UID       string
	Title     string
	StartAt   int64 // unix seconds
	EndAt     int64 // unix seconds
	AllDay    bool
	Location  string
	Status    string
	SyncedAt  int64
}
