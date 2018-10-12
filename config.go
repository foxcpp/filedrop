package filedrop

import "net/http"

type LimitsConfig struct {
	MaxUses uint
	MaxStoreSecs uint
	MaxFileSize uint
}

type DBConfig struct {
	Driver string
	DSN string
}

type AuthConfig struct {
	//Query string TODO

	// Used only in embedded interface.
	Callback func(*http.Request) bool `yaml:"omitempty"`
}

type Config struct {
	Limits LimitsConfig
	DB DBConfig
	DownloadAuth AuthConfig
	UploadAuth AuthConfig
	StorageDir string
	HTTPSUpstream bool
}

var Default Config