# hqurlfind3r

[![release](https://img.shields.io/github/release/hueristiq/hqurlfind3r?style=flat&color=0040ff)](https://github.com/hueristiq/hqurlfind3r/releases) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-0040ff.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hqurlfind3r.svg?style=flat&color=0040ff)](https://github.com/hueristiq/hqurlfind3r/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hqurlfind3r.svg?style=flat&color=0040ff)](https://github.com/hueristiq/hqurlfind3r/issues?q=is:issue+is:closed) [![license](https://img.shields.io/badge/license-MIT-gray.svg?colorB=0040FF)](https://github.com/hueristiq/hqurlfind3r/blob/master/LICENSE) [![twitter](https://img.shields.io/badge/twitter-@itshueristiq-0040ff.svg)](https://twitter.com/itshueristiq)

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

You can download the pre-built binary for your platform from this repository's [releases](https://github.com/hueristiq/hqurlfind3r/releases/) page, extract, then move it to your `$PATH`and you're ready to go.

### From Source

hqurlfind3r requires **go1.17+** to install successfully. Run the following command to get the repo

```bash
go install -v github.com/hueristiq/hqurlfind3r/cmd/hqurlfind3r@latest
```

### From Github

```bash
git clone https://github.com/hueristiq/hqurlfind3r.git && \
cd hqurlfind3r/cmd/hqurlfind3r/ && \
go build; mv hqurlfind3r /usr/local/bin/ && \
hqurlfind3r -h
```

## Post Installation

hqurlfind3r will work after [installation](#installation). However, to configure hqurlfind3r to work with certain services - currently github - you will need to have setup API keys. The API keys are stored in the `$HOME/.config/hqurlfind3r/conf.yaml` file - created upon first run - and uses the YAML format. Multiple API keys can be specified for each of these services.

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
hqurlfind3r -h
```

This will display help for the tool.

```
 _                      _  __ _           _ _____      
| |__   __ _ _   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
| '_ \ / _` | | | | '__| | |_| | '_ \ / _` | |_ \| '__|
| | | | (_| | |_| | |  | |  _| | | | | (_| |___) | |   
|_| |_|\__, |\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_| v1.8.0
          |_|
USAGE:
  hqurlfind3r [OPTIONS]

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
hqurlfind3r -d tesla.com
```

#### Regex filter URLs

```bash
hqurlfind3r -d tesla.com -f ".(jpg|jpeg|gif|png|ico|css|eot|tif|tiff|ttf|woff|woff2)"
```

#### Include Subdomains' URLs

```bash
hqurlfind3r -d tesla.com -iS
```

## Contribution

[Issues](https://github.com/hueristiq/hqurlfind3r/issues) and [Pull Requests](https://github.com/hueristiq/hqurlfind3r/pulls) are welcome!
