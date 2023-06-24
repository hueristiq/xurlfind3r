package configuration

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"dario.cat/mergo"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v3"
	"gopkg.in/yaml.v3"
)

type Configuration struct {
	Version string   `yaml:"version"`
	Sources []string `yaml:"sources"`
	Keys    Keys     `yaml:"keys"`
}

func (configuration *Configuration) GetKeys() sources.Keys {
	keys := sources.Keys{}

	// Github
	if len(configuration.Keys.Github) > 0 {
		keys.GitHub = configuration.Keys.Github
	}

	// IntelX
	intelxKeysCount := len(configuration.Keys.Intelx)
	if intelxKeysCount > 0 {
		intelxKeys := configuration.Keys.Intelx[rand.Intn(intelxKeysCount)] //nolint:gosec // Works perfectly
		parts := strings.Split(intelxKeys, ":")

		if len(parts) == 2 {
			keys.IntelXHost = parts[0]
			keys.IntelXKey = parts[1]
		}
	}

	// URLScan
	if len(configuration.Keys.URLScan) > 0 {
		keys.URLScan = configuration.Keys.URLScan
	}

	return keys
}

func (configuration *Configuration) Write(path string) (err error) {
	var (
		file *os.File
	)

	directory := filepath.Dir(path)
	identation := 4

	if _, err = os.Stat(directory); os.IsNotExist(err) {
		if directory != "" {
			if err = os.MkdirAll(directory, os.ModePerm); err != nil {
				return
			}
		}
	}

	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return
	}

	defer file.Close()

	enc := yaml.NewEncoder(file)
	enc.SetIndent(identation)
	err = enc.Encode(&configuration)

	return
}

type Keys struct {
	Github  []string `yaml:"github"`
	Intelx  []string `yaml:"intelx"`
	URLScan []string `yaml:"urlscan"`
}

const (
	NAME    string = "xurlfind3r"
	VERSION string = "0.1.0"
)

var (
	BANNER = aurora.Sprintf(
		aurora.BrightBlue(`
                 _  __ _           _ _____      
__  ___   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
\ \/ / | | | '__| | |_| | '_ \ / _`+"`"+` | |_ \| '__|
 >  <| |_| | |  | |  _| | | | | (_| |___) | |
/_/\_\\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_| %s
`).Bold(),
		aurora.BrightYellow("v"+VERSION).Bold(),
	)
)

func CreateUpdate(path string) (err error) {
	var (
		config Configuration
	)

	defaultConfig := Configuration{
		Version: VERSION,
		Sources: sources.List,
		Keys: Keys{
			Github:  []string{},
			Intelx:  []string{},
			URLScan: []string{},
		},
	}

	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			config = defaultConfig

			if err = config.Write(path); err != nil {
				return
			}
		} else {
			return
		}
	} else {
		config, err = Read(path)
		if err != nil {
			return
		}

		if config.Version != VERSION ||
			len(config.Sources) != len(sources.List) {
			if err = mergo.Merge(&config, defaultConfig); err != nil {
				return
			}

			config.Version = VERSION
			config.Sources = sources.List

			if err = config.Write(path); err != nil {
				return
			}
		}
	}

	return
}

func Read(path string) (configuration Configuration, err error) {
	var (
		file *os.File
	)

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
