package filedrop

import "net/http"

type LimitsConfig struct {
	// MaxUses is how much much times file can be accessed. Note that it also counts HEAD requests
	// and incomplete downloads (byte-range requests).
	// Per-file max-uses parameter can't exceed this value but can be smaller.
	MaxUses  uint `yaml:"max_uses"`

	// MaxStoreSecs specifies max time for which files will be stored on
	// filedrop server. Per-file store-secs parameter can't exceed this value but
	// can be smaller.
	MaxStoreSecs uint `yaml:"max_store_secs"`

	// MaxFileSize is a maximum file size in bytes that can uploaded to filedrop.
	MaxFileSize  uint `yaml:"max_file_size"`
}

type DBConfig struct {
	// Driver is a database/sql driver name.
	Driver string `yaml:"driver"`

	// Data Source Name.
	DSN  string `yaml:"dsn"`
}

type AuthConfig struct {
	// Callback is called to check access before processing any request.
	// If Callback is null, no check will be performed.
	Callback func(*http.Request) bool `yaml:"omitempty"`
}

type Config struct {
	// ListenOn specifies endpoint to listen on in format ADDR:PORT. Used only by filedropd.
	ListenOn string `yaml:"listen_on"`

	Limits          LimitsConfig `yaml:"limits"`
	DB              DBConfig     `yaml:"db"`
	DownloadAuth    AuthConfig   `yaml:"download_auth"`
	UploadAuth      AuthConfig   `yaml:"upload_auth"`

	// StorageDir is where files will be saved on disk.
	StorageDir  string  `yaml:"storage_dir"`

	// HTTPSDownstream specifies whether filedrop should return links with https scheme or not.
	// Overridden by X-HTTPS-Downstream header.
	HTTPSDownstream bool `yaml:"https_downstream"`

	// AllowedOrigins specifies Access-Control-Allow-Origin header.
	AllowedOrigins string `yaml:"allowed_origins"`

	// Internal, used only for testing. Always 60 secs in production.
	CleanupIntervalSecs int `yaml:"-"`
}

var Default Config
