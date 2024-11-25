package configuration

import (
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"github.com/hueristiq/hqgolog"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v3"
	"gopkg.in/yaml.v3"
)

// Configuration represents the core utility settings.
// It is structured to support extensibility and ease of management.
//
// Fields:
// - Version: Specifies the configuration schema's version, aiding compatibility checks.
// - Sources: Lists source configurations for external integrations or data sources.
// - Keys: Holds API keys for various services to be utilized by the utility.
type Configuration struct {
	Version string       `yaml:"version"`
	Sources []string     `yaml:"sources"`
	Keys    sources.Keys `yaml:"keys"`
}

// Write persists the current configuration into a YAML file.
// It ensures that the target directory exists and handles file creation
// with the correct permissions.
//
// Parameters:
//   - path string: The file system path where the configuration will be stored.
//
// Returns:
//   - err error: Returns an error if writing the configuration fails, otherwise nil.
func (configuration *Configuration) Write(path string) (err error) {
	var file *os.File

	directory := filepath.Dir(path)
	identation := 4

	if _, err = os.Stat(directory); os.IsNotExist(err) {
		if directory != "" {
			if err = os.MkdirAll(directory, os.ModePerm); err != nil {
				return
			}
		}
	}

	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return
	}

	defer file.Close()

	enc := yaml.NewEncoder(file)
	enc.SetIndent(identation)
	err = enc.Encode(&configuration)

	return
}

const (
	// NAME is the utility identifier used in configuration and branding.
	NAME = "xurlfind3r"
	// VERSION specifies the current version of the utility.
	VERSION = "0.6.0"
)

var (
	// BANNER provides a visually formatted utility banner for display.
	BANNER = aurora.Sprintf(
		aurora.BrightBlue(`
                 _  __ _           _ _____      
__  ___   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
\ \/ / | | | '__| | |_| | '_ \ / _`+"`"+` | |_ \| '__|
 >  <| |_| | |  | |  _| | | | | (_| |___) | |
/_/\_\\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_|
                                          %s`).Bold(),
		aurora.BrightRed("v"+VERSION).Bold(),
	)
	// UserDotConfigDirectoryPath returns the user's configuration directory path,
	// ensuring the utility can save configuration files in a standard location.
	UserDotConfigDirectoryPath = func() (userDotConfig string) {
		var err error

		userDotConfig, err = os.UserConfigDir()
		if err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}

		return
	}()
	// DefaultConfigurationFilePath defines the default location for the utility's configuration file.
	DefaultConfigurationFilePath = filepath.Join(UserDotConfigDirectoryPath, NAME, "config.yaml")
	// DefaultConfiguration provides a pre-configured instance with default values.
	// It includes default sources and empty API keys for services.
	DefaultConfiguration = &Configuration{
		Version: VERSION,
		Sources: sources.List,
		Keys: sources.Keys{
			Bevigil: []string{},
			Github:  []string{},
			IntelX:  []string{},
			URLScan: []string{},
		},
	}
)

// CreateOrUpdate ensures a configuration file exists at the specified path.
// If the file does not exist, it creates one using default settings.
// If the file exists but is outdated or missing settings, it updates the file.
//
// Parameters:
//   - path string: The file path where the configuration will be checked or updated.
//
// Returns:
//   - err error: Returns an error if the process fails, otherwise nil.
func CreateUpdate(path string) (err error) {
	var cfg *Configuration

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

		if cfg.Version != VERSION || len(cfg.Sources) != len(sources.List) {
			if err = mergo.Merge(&cfg, DefaultConfiguration); err != nil {
				return
			}

			cfg.Version = VERSION
			cfg.Sources = sources.List

			if err = cfg.Write(path); err != nil {
				return
			}
		}
	}

	return
}

// Read loads a YAML configuration file from the specified path.
// It initializes a Configuration struct with the values found in the file.
//
// Parameters:
//   - path string: The file path from which the configuration will be loaded.
//
// Returns:
//   - cfg *Configuration: A pointer to the loaded Configuration instance.
//   - err error: Returns an error if reading the file or parsing the YAML fails.
func Read(path string) (configuration *Configuration, err error) {
	var file *os.File

	file, err = os.Open(path)
	if err != nil {
		return
	}

	defer file.Close()

	if err = yaml.NewDecoder(file).Decode(&configuration); err != nil {
		return
	}

	return
}
