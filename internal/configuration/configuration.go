package configuration

import (
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	hqgologger "github.com/hueristiq/hq-lib-logger-go"
	"github.com/logrusorgru/aurora/v4"
	"gopkg.in/yaml.v3"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

// Name is the utility's name.
const Name = "xurlfind3r"

// Version is the utility's version. It is stamped at build time via
// -ldflags and defaults to the last release.
var Version = "1.3.0"

// Configuration is the utility's persisted configuration.
//
// Fields:
//   - Version (string): the configuration schema version.
//   - Sources ([]string): the default sources to use.
//   - Keys (map[string]sources.Keys): per-source API keys.
type Configuration struct {
	Version string                  `yaml:"version"`
	Sources []string                `yaml:"sources"`
	Keys    map[string]sources.Keys `yaml:"keys"`
}

// Write persists the configuration to path, creating parent directories
// as needed.
//
// Parameters:
//   - path (string): the configuration file path.
//
// Returns:
//   - err (error): non-nil on failure.
func (configuration *Configuration) Write(path string) (err error) {
	var file *os.File

	directory := filepath.Dir(path)
	indentation := 4

	if _, err = os.Stat(directory); os.IsNotExist(err) {
		if directory != "" {
			if err = os.MkdirAll(directory, 0o750); err != nil {
				return
			}
		}
	}

	//nolint:gosec // the path is derived from the user's configuration directory; the file is created with 0600 permissions.
	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}

	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed closing file: %w", cerr)
		}
	}()

	enc := yaml.NewEncoder(file)

	enc.SetIndent(indentation)

	err = enc.Encode(configuration)

	return
}

// Banner renders the utility's ASCII-art banner.
//
// Parameters:
//   - au (*aurora.Aurora): the colorizer.
//
// Returns:
//   - banner (string): the rendered banner.
func Banner(au *aurora.Aurora) (banner string) {
	banner = au.Sprintf(
		au.BrightBlue(`
                 _  __ _           _ _____      
__  ___   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
\ \/ / | | | '__| | |_| | '_ \ / _`+"`"+` | |_ \| '__|
 >  <| |_| | |  | |  _| | | | | (_| |___) | |
/_/\_\\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_|
                                          %s`).Bold(),
		au.BrightRed("v"+Version).Bold().Italic(),
	) + "\n\n"

	return
}

// UserDotConfigDirectoryPath returns the user's configuration directory
// (~/.config).
//
// Returns:
//   - userDotConfig (string): the configuration directory path.
func UserDotConfigDirectoryPath() (userDotConfig string) {
	var err error

	userDotConfig, err = os.UserConfigDir()
	if err != nil {
		hqgologger.Fatal("failed to get `$HOME/.config/`", hqgologger.WithError(err))
	}

	return
}

// DefaultConfigurationFilePath is the default configuration file path.
var DefaultConfigurationFilePath = filepath.Join(UserDotConfigDirectoryPath(), Name, "config.yaml")

// DefaultConfiguration is the configuration written on first run.
var DefaultConfiguration = Configuration{
	Version: Version,
	Sources: sources.List,
	Keys: map[string]sources.Keys{
		sources.Bevigil:       {},
		sources.Github:        {},
		sources.Gitlab:        {},
		sources.IntelligenceX: {},
		sources.Leakradar:     {},
		sources.URLScan:       {},
		sources.Virustotal:    {},
		sources.Zoomeye:       {},
	},
}

// CreateUpdate creates the configuration file at path when missing, and
// migrates it to the current schema otherwise.
//
// Parameters:
//   - path (string): the configuration file path.
//
// Returns:
//   - err (error): non-nil on failure.
func CreateUpdate(path string) (err error) {
	var cfg Configuration

	_, err = os.Stat(path)

	switch {
	case err != nil && os.IsNotExist(err):
		cfg = DefaultConfiguration

		if err = cfg.Write(path); err != nil {
			return
		}
	case err != nil:
		return
	default:
		cfg, err = Read(path)
		if err != nil {
			return
		}

		if cfg.Version != Version || len(cfg.Sources) != len(sources.List) {
			if err = mergo.Merge(&cfg, DefaultConfiguration); err != nil {
				return
			}

			cfg.Version = Version
			cfg.Sources = sources.List

			if err = cfg.Write(path); err != nil {
				return
			}
		}
	}

	return
}

// Read loads the configuration at path.
//
// Parameters:
//   - path (string): the configuration file path.
//
// Returns:
//   - configuration (Configuration): the loaded configuration.
//   - err (error): non-nil on failure.
func Read(path string) (configuration Configuration, err error) {
	var file *os.File

	//nolint:gosec // the path is user-controlled via the --configuration flag; only the user's own config file is read.
	file, err = os.Open(path)
	if err != nil {
		return
	}

	defer func() {
		// Close failures on a read-only config file are not actionable.
		_ = file.Close()
	}()

	if err = yaml.NewDecoder(file).Decode(&configuration); err != nil {
		return
	}

	return
}
