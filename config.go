package filedrop

import "net/http"

type LimitsConfig struct {
	MaxUses      uint `yaml:"max_uses"`
	MaxStoreSecs uint `yaml:"max_store_secs"`
	MaxFileSize  uint `yaml:"max_file_size"`
}

type DBConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type AuthConfig struct {
	//Query string TODO

	// Used only in embedded interface.
	Callback func(*http.Request) bool `yaml:"omitempty"`
}

type Config struct {
	Limits          LimitsConfig `yaml:"limits"`
	DB              DBConfig     `yaml:"db"`
	DownloadAuth    AuthConfig   `yaml:"download_auth"`
	UploadAuth      AuthConfig   `yaml:"upload_auth"`
	StorageDir      string       `yaml:"storage_dir"`
	HTTPSDownstream bool         `yaml:"https_downstream"`
}

var Default Config
