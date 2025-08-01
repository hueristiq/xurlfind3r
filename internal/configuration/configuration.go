package configuration

import (
	"os"
	"path/filepath"

	"dario.cat/mergo"
	hqgologger "github.com/hueristiq/hq-go-logger"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v4"
	"gopkg.in/yaml.v3"
)

type Configuration struct {
	Version string       `yaml:"version"`
	Sources []string     `yaml:"sources"`
	Keys    sources.Keys `yaml:"keys"`
}

func (configuration *Configuration) Write(path string) (err error) {
	var file *os.File

	directory := filepath.Dir(path)
	identation := 4

	if _, err = os.Stat(directory); os.IsNotExist(err) {
		if directory != "" {
			if err = os.MkdirAll(directory, 0o750); err != nil {
				return
			}
		}
	}

	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
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
	NAME    = "xurlfind3r"
	VERSION = "1.2.0"
)

var (
	BANNER = func(au *aurora.Aurora) (banner string) {
		banner = au.Sprintf(
			au.BrightBlue(`
                 _  __ _           _ _____      
__  ___   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
\ \/ / | | | '__| | |_| | '_ \ / _`+"`"+` | |_ \| '__|
 >  <| |_| | |  | |  _| | | | | (_| |___) | |
/_/\_\\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_|
                                          %s`).Bold(),
			au.BrightRed("v"+VERSION).Bold().Italic(),
		) + "\n\n"

		return
	}

	UserDotConfigDirectoryPath = func() (userDotConfig string) {
		var err error

		userDotConfig, err = os.UserConfigDir()
		if err != nil {
			hqgologger.Fatal("failed to get `$HOME/.config/`", hqgologger.WithError(err))
		}

		return
	}()

	DefaultConfigurationFilePath = filepath.Join(UserDotConfigDirectoryPath, NAME, "config.yaml")
	DefaultConfiguration         = Configuration{
		Version: VERSION,
		Sources: sources.List,
		Keys: sources.Keys{
			Bevigil:    []string{},
			Github:     []string{},
			IntelX:     []string{},
			URLScan:    []string{},
			VirusTotal: []string{},
		},
	}
)

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

func Read(path string) (configuration Configuration, err error) {
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
