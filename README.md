# urlfind3r

[![release](https://img.shields.io/github/release/hueristiq/urlfind3r?style=flat&color=0040ff)](https://github.com/hueristiq/urlfind3r/releases) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-0040ff.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/urlfind3r.svg?style=flat&color=0040ff)](https://github.com/hueristiq/urlfind3r/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/urlfind3r.svg?style=flat&color=0040ff)](https://github.com/hueristiq/urlfind3r/issues?q=is:issue+is:closed) [![license](https://img.shields.io/badge/license-MIT-gray.svg?colorB=0040FF)](https://github.com/hueristiq/urlfind3r/blob/master/LICENSE) [![twitter](https://img.shields.io/badge/twitter-@itshueristiq-0040ff.svg)](https://twitter.com/itshueristiq)

A passive reconnaissance tool for known URLs discovery - it gathers a list of URLs passively using various online sources.

## Resource

* [Features](#features)
* [Installation](#installation)
	* [From Binary](#from-binary)
	* [From source](#from-source)
	* [From github](#from-github)
* [Post Installation](#post-installation)
* [Usage](#usage)
	* [Examples](#examples)
		* [Basic](#basic)
		* [Regex filter URLs](#regex-filter-urls)
		* [Include Subdomains' URLs](#include-subdomains-urls)
* [Contribution](#contribution)

## Features

* Collect known URLs:
    * Fetches from **[AlienVault's OTX](https://otx.alienvault.com/)**, **[Common Crawl](https://commoncrawl.org/)**, **[URLScan](https://urlscan.io/)**, **[Github](https://github.com)**, **[Intelligence X](https://intelx.io)** and the **[Wayback Machine](https://archive.org/web/)**.
    * Fetches disallowed paths from `robots.txt` found on your target domain and snapshotted by the Wayback Machine.
* Reduce noise:
    * Regex filter URLs.
    * Removes duplicate pages in the sense of URL patterns that are probably repetitive and points to the same web template.
* Output to stdout for piping or save to file.

## Installation

### From Binary

You can download the pre-built binary for your platform from this repository's [releases](https://github.com/hueristiq/urlfind3r/releases/) page, extract, then move it to your `$PATH`and you're ready to go.

### From Source

urlfind3r requires **go1.17+** to install successfully. Run the following command to get the repo

```bash
go install -v github.com/hueristiq/urlfind3r/cmd/urlfind3r@latest
```

### From Github

```bash
git clone https://github.com/hueristiq/urlfind3r.git && \
cd urlfind3r/cmd/urlfind3r/ && \
go build; mv urlfind3r /usr/local/bin/ && \
urlfind3r -h
```

## Post Installation

urlfind3r will work after [installation](#installation). However, to configure urlfind3r to work with certain services - currently github - you will need to have setup API keys. The API keys are stored in the `$HOME/.config/urlfind3r/conf.yaml` file - created upon first run - and uses the YAML format. Multiple API keys can be specified for each of these services.

Example:

```yaml
version: 1.8.0
sources:
    - commoncrawl
    - github
    - intelx
    - otx
    - urlscan
    - wayback
    - waybackrobots
keys:
    github:
        - d23a554bbc1aabb208c9acfbd2dd41ce7fc9db39
        - asdsd54bbc1aabb208c9acfbd2dd41ce7fc9db39
    intelx:
        - 2.intelx.io:00000000-0000-0000-0000-000000000000
```

## Usage

**DiSCLAIMER:** fetching urls from github is a bit slow.

```bash
urlfind3r -h
```

This will display help for the tool.

```
            _  __ _           _ _____      
 _   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
| | | | '__| | |_| | '_ \ / _` | |_ \| '__|
| |_| | |  | |  _| | | | | (_| |___) | |   
 \__,_|_|  |_|_| |_|_| |_|\__,_|____/|_| 1.8.0

USAGE:
  urlfind3r [OPTIONS]

OPTIONS:
   -d, --domain            domain to fetch urls for
  -eS, --exclude-sources   comma(,) separated list of sources to exclude
   -f, --filter            URL filtering regex
  -iS, --include-subs      include subdomains' urls
  -lS, --list-sources      list all the available sources
  -nC, --no-color          no color mode
   -s  --silent            silent mode: output urls only
  -uS, --use-sources       comma(,) separated list of sources to use
   -o, --output            output file
```

### Examples

#### Basic

```bash
urlfind3r -d tesla.com
```

#### Regex filter URLs

```bash
urlfind3r -d tesla.com -f ".(jpg|jpeg|gif|png|ico|css|eot|tif|tiff|ttf|woff|woff2)"
```

#### Include Subdomains' URLs

```bash
urlfind3r -d tesla.com -iS
```

## Contribution

[Issues](https://github.com/hueristiq/urlfind3r/issues) and [Pull Requests](https://github.com/hueristiq/urlfind3r/pulls) are welcome!
