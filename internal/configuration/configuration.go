package configuration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"github.com/logrusorgru/aurora/v4"
	"gopkg.in/yaml.v3"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type Configuration struct {
	Version string                  `yaml:"version"`
	Sources []string                `yaml:"sources"`
	Keys    map[string]sources.Keys `yaml:"keys"`
}

func (cfg *Configuration) Write(path string) (err error) {
	directory := filepath.Dir(path)
	indentation := 4

	if _, err = os.Stat(directory); errors.Is(err, os.ErrNotExist) {
		if directory != "" {
			if err = os.MkdirAll(directory, 0o750); err != nil {
				return
			}
		}
	}

	var file *os.File

	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // path is the user-specified configuration file location.
	if err != nil {
		return
	}

	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	enc := yaml.NewEncoder(file)

	enc.SetIndent(indentation)

	if err = enc.Encode(cfg); err != nil {
		return
	}

	if err = enc.Close(); err != nil {
		return
	}

	return
}

const Name = "xurlfind3r"

var Version = "1.3.0"

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

func UserConfigDirectoryPath() (userConfigDirectoryPath string, err error) {
	userConfigDirectoryPath, err = os.UserConfigDir()
	if err != nil {
		err = fmt.Errorf("failed to resolve user configuration directory: %w", err)

		return
	}

	return
}

func DefaultConfigurationFilePath() (path string) {
	userConfigDirectoryPath, err := UserConfigDirectoryPath()
	if err != nil {
		userConfigDirectoryPath = "."
	}

	path = filepath.Join(userConfigDirectoryPath, Name, "config.yaml")

	return
}

var DefaultConfiguration = Configuration{
	Version: Version,
	Sources: sources.List,
	Keys: map[string]sources.Keys{
		sources.Bevigil:            {},
		sources.Github:             {},
		sources.Gitlab:             {},
		sources.IntelligenceX:      {},
		sources.Leakradar:          {},
		sources.OpenThreatExchange: {},
		sources.URLScan:            {},
		sources.Virustotal:         {},
		sources.Zoomeye:            {},
	},
}

func CreateOrUpdate(path string) (err error) {
	var cfg Configuration

	_, err = os.Stat(path)

	switch {
	case errors.Is(err, os.ErrNotExist):
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

		if cfg.Version != Version || len(cfg.Sources) != len(sources.List) || hasMissingKeySlots(&cfg) {
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

func hasMissingKeySlots(cfg *Configuration) (missing bool) {
	for name := range DefaultConfiguration.Keys {
		if _, exists := cfg.Keys[name]; !exists {
			return true
		}
	}

	return
}

func Read(path string) (cfg Configuration, err error) {
	file, err := os.Open(path) //nolint:gosec // path is the user-specified configuration file location.
	if err != nil {
		return
	}

	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	if err = yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return
	}

	return
}
