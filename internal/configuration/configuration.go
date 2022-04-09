package configuration

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"strings"

	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
	"gopkg.in/yaml.v3"
)

type YAMLConfiguration struct {
	Version string   `yaml:"version"`
	Sources []string `yaml:"sources"`
	Keys    struct {
		GitHub []string `yaml:"github"`
		Intelx []string `yaml:"intelx"`
	}
}

type CLIOptions struct {
	Domain            string
	FilterRegex       string
	IncludeSubdomains bool
	ListSources       bool
	SourcesToExclude  string
	SourcesToUse      string
}

type Options struct {
	Domain            string
	FilterRegex       string
	IncludeSubdomains bool
	ListSources       bool
	SourcesToExclude  []string
	SourcesToUse      []string
	YAML              YAMLConfiguration
}

const (
	VERSION = "1.8.0"
)

var (
	BANNER string = fmt.Sprintf(`
     _                  _  __ _           _ _____
 ___(_) __ _ _   _ _ __| |/ _(_)_ __   __| |___ / _ __
/ __| |/ _`+"`"+` | | | | '__| | |_| | '_ \ / _`+"`"+` | |_ \| '__|
\__ \ | (_| | |_| | |  | |  _| | | | | (_| |___) | |
|___/_|\__, |\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_| %s
       |___/
`, VERSION)
)

// ParseCLIOptions parse the command line flags and read config file
func ParseCLIOptions(options *CLIOptions) (parsedOptions *Options, err error) {
	directory, err := os.UserHomeDir()
	if err != nil {
		return
	}

	configPath := directory + "/.config/sigurlfind3r/conf.yaml"

	parsedOptions = &Options{
		Domain:            options.Domain,
		FilterRegex:       options.FilterRegex,
		IncludeSubdomains: options.IncludeSubdomains,
		ListSources:       options.ListSources,
	}

	if options.SourcesToUse != "" {
		parsedOptions.SourcesToUse = append(parsedOptions.SourcesToUse, strings.Split(options.SourcesToUse, ",")...)
	} else {
		parsedOptions.SourcesToUse = append(parsedOptions.SourcesToUse, scraping.SourcesList...)
	}

	if options.SourcesToExclude != "" {
		parsedOptions.SourcesToExclude = append(parsedOptions.SourcesToExclude, strings.Split(options.SourcesToExclude, ",")...)
	}

	if _, err = os.Stat(configPath); os.IsNotExist(err) {
		configuration := YAMLConfiguration{
			Version: VERSION,
			Sources: scraping.SourcesList,
		}

		directory, _ := path.Split(configPath)

		if err = makeDirectory(directory); err != nil {
			return
		}

		if err = configuration.MarshalWrite(configPath); err != nil {
			return
		}

		parsedOptions.YAML = configuration
	} else {
		configuration, err := UnmarshalRead(configPath)
		if err != nil {
			return nil, err
		}

		if configuration.Version != VERSION {
			configuration.Sources = scraping.SourcesList
			configuration.Version = VERSION

			if err = configuration.MarshalWrite(configPath); err != nil {
				return nil, err
			}
		}

		parsedOptions.YAML = configuration
	}

	return
}

func makeDirectory(directory string) error {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		if directory != "" {
			err = os.MkdirAll(directory, os.ModePerm)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (config *YAMLConfiguration) MarshalWrite(file string) error {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	enc := yaml.NewEncoder(f)
	enc.SetIndent(4)
	err = enc.Encode(&config)
	f.Close()
	return err
}

func UnmarshalRead(file string) (YAMLConfiguration, error) {
	config := YAMLConfiguration{}

	f, err := os.Open(file)
	if err != nil {
		return config, err
	}

	err = yaml.NewDecoder(f).Decode(&config)

	f.Close()

	return config, err
}

func (config *YAMLConfiguration) GetKeys() session.Keys {
	keys := session.Keys{}

	if len(config.Keys.GitHub) > 0 {
		keys.GitHub = config.Keys.GitHub
	}

	intelxKeysCount := len(config.Keys.Intelx)
	if intelxKeysCount > 0 {
		intelxKeys := config.Keys.Intelx[rand.Intn(intelxKeysCount)]
		parts := strings.Split(intelxKeys, ":")
		if len(parts) == 2 {
			keys.IntelXHost = parts[0]
			keys.IntelXKey = parts[1]
		}
	}

	return keys
}
