package configuration

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	hqlog "github.com/hueristiq/hqgoutils/log"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v3"
	"gopkg.in/yaml.v3"
)

type Keys struct {
	Github []string `yaml:"github"`
	Intelx []string `yaml:"intelx"`
}

type Configuration struct {
	Version string   `yaml:"version"`
	Sources []string `yaml:"sources"`
	Keys    Keys     `yaml:"keys"`
}

const (
	NAME        string = "xurlfind3r"
	VERSION     string = "0.1.0"
	DESCRIPTION string = "A CLI utility to find domain's known URLs."
)

var (
	BANNER = aurora.Sprintf(
		aurora.BrightBlue(`
                 _  __ _           _ _____      
__  ___   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
\ \/ / | | | '__| | |_| | '_ \ / _`+"`"+` | |_ \| '__|
 >  <| |_| | |  | |  _| | | | | (_| |___) | |
/_/\_\\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_| %s

%s
`).Bold(),
		aurora.BrightYellow("v"+VERSION).Bold(),
		aurora.BrightGreen(DESCRIPTION).Italic(),
	)
	rootDirectoryName        = ".hueristiq"
	projectRootDirectoryName = NAME
	ProjectRootDirectoryPath = func(rootDirectoryName, projectRootDirectoryName string) string {
		home, err := os.UserHomeDir()
		if err != nil {
			hqlog.Fatal().Msg(err.Error())
		}

		return filepath.Join(home, rootDirectoryName, projectRootDirectoryName)
	}(rootDirectoryName, projectRootDirectoryName)
	configurationFileName = "config.yaml"
	ConfigurationFilePath = filepath.Join(ProjectRootDirectoryPath, configurationFileName)
	Default               = Configuration{
		Version: VERSION,
		Sources: sources.List,
		Keys: Keys{
			Github: []string{},
			Intelx: []string{},
		},
	}
)

func Read() (configuration Configuration, err error) {
	var (
		file *os.File
	)

	file, err = os.Open(ConfigurationFilePath)
	if err != nil {
		return
	}

	defer file.Close()

	err = yaml.NewDecoder(file).Decode(&configuration)

	return
}

func Write(configuration *Configuration) (err error) {
	var (
		file       *os.File
		identation = 4
	)

	directory := filepath.Dir(ConfigurationFilePath)

	if _, err = os.Stat(directory); os.IsNotExist(err) {
		if directory != "" {
			if err = os.MkdirAll(directory, os.ModePerm); err != nil {
				return
			}
		}
	}

	file, err = os.OpenFile(ConfigurationFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return
	}

	defer file.Close()

	enc := yaml.NewEncoder(file)
	enc.SetIndent(identation)
	err = enc.Encode(&configuration)

	return
}

func (configuration *Configuration) GetKeys() sources.Keys {
	keys := sources.Keys{}

	if len(configuration.Keys.Github) > 0 {
		keys.GitHub = configuration.Keys.Github
	}

	intelxKeysCount := len(configuration.Keys.Intelx)
	if intelxKeysCount > 0 {
		intelxKeys := configuration.Keys.Intelx[rand.Intn(intelxKeysCount)] //nolint:gosec // Works perfectly
		parts := strings.Split(intelxKeys, ":")

		if len(parts) == 2 {
			keys.IntelXHost = parts[0]
			keys.IntelXKey = parts[1]
		}
	}

	return keys
}
