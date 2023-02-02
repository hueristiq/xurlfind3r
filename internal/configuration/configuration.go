package configuration

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources"
	"gopkg.in/yaml.v3"
)

type Configuration struct {
	Version string   `yaml:"version"`
	Sources []string `yaml:"sources"`
	Keys    struct {
		GitHub []string `yaml:"github"`
		Intelx []string `yaml:"intelx"`
	}
}

const (
	VERSION = "1.9.0"
)

var (
	BANNER string = fmt.Sprintf(`
 _                      _  __ _           _ _____      
| |__   __ _ _   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
| '_ \ / _`+"`"+` | | | | '__| | |_| | '_ \ / _`+"`"+` | |_ \| '__|
| | | | (_| | |_| | |  | |  _| | | | | (_| |___) | |   
|_| |_|\__, |\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_| v%s
          |_|
`, VERSION)
	ConfigurationFilePath = func() string {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalln(err)
		}

		return filepath.Join(home, "/.config/hqurlfind3r/conf.yaml")
	}()
)

func (configuration *Configuration) Write() error {
	file, err := os.OpenFile(ConfigurationFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	enc := yaml.NewEncoder(file)
	enc.SetIndent(4)
	err = enc.Encode(&configuration)
	file.Close()
	return err
}

func Read() (Configuration, error) {
	configuration := Configuration{}

	file, err := os.Open(ConfigurationFilePath)
	if err != nil {
		return configuration, err
	}

	err = yaml.NewDecoder(file).Decode(&configuration)

	file.Close()

	return configuration, err
}

func (configuration *Configuration) GetKeys() sources.Keys {
	keys := sources.Keys{}

	if len(configuration.Keys.GitHub) > 0 {
		keys.GitHub = configuration.Keys.GitHub
	}

	intelxKeysCount := len(configuration.Keys.Intelx)
	if intelxKeysCount > 0 {
		intelxKeys := configuration.Keys.Intelx[rand.Intn(intelxKeysCount)]
		parts := strings.Split(intelxKeys, ":")
		if len(parts) == 2 {
			keys.IntelXHost = parts[0]
			keys.IntelXKey = parts[1]
		}
	}

	return keys
}
