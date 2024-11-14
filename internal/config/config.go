package config

type Configuration struct {
	SqliteFile string `env:"SQLITE_FILE, required"`

	AuthRedditAppId       string `env:"AUTH_ID, required"`
	AuthRedditAppSecret   string `env:"AUTH_SECRET, required"`
	AuthRedditRedirectUrl string `env:"AUTH_REDIRECTURL, required"`
}
