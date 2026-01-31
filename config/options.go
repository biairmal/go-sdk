package config

// options holds configuration for Load. It is populated by Option functions.
type options struct {
	envFile string
	files   []string
}

// Option configures Load behaviour. Options are applied in order; later
// options override earlier ones for the same field.
type Option func(*options)

// EnvFile sets the path to a .env file to load before reading config files.
// Path is relative to the current working directory or absolute. If empty,
// no .env file is loaded. If the file does not exist, Load returns an error
// unless the caller uses LoadEnvFileOptional elsewhere or the file is optional.
func EnvFile(path string) Option {
	return func(o *options) {
		o.envFile = path
	}
}

// Files sets the config file paths to read in order. The first file is the
// base; subsequent files are merged over it (later keys override). Each file
// is read, has ${VAR} and ${VAR:default} substituted, then is fed to Viper.
func Files(paths ...string) Option {
	return func(o *options) {
		o.files = paths
	}
}
