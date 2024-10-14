# X URLs Finder (`xurlfind3r`)

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go report card](https://goreportcard.com/badge/github.com/hueristiq/xurlfind3r)](https://goreportcard.com/report/github.com/hueristiq/xurlfind3r) [![release](https://img.shields.io/github/release/hueristiq/xurlfind3r?style=flat&color=1E90FF)](https://github.com/hueristiq/xurlfind3r/releases) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/xurlfind3r.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/xurlfind3r/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/xurlfind3r.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/xurlfind3r/issues?q=is:issue+is:closed) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/xurlfind3r/blob/master/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![contribution](https://img.shields.io/badge/contributions-welcome-1E90FF.svg)](https://github.com/hueristiq/xurlfind3r/blob/master/CONTRIBUTING.md)

`xurlfind3r` is a command-line utility designed to discover URLs for a given domain in a simple, efficient way. It works by gathering information from a variety of passive sources, meaning it doesn't interact directly with the target but instead gathers data that is already publicly available. This makes `xurlfind3r` a powerful tool for security researchers, IT professionals, and anyone looking to gain insights into the URLs associated with a domain.

## Resource

* [Features](#features)
* [Installation](#installation)
	* [Install release binaries (Without Go Installed)](#install-release-binaries-without-go-installed)
	* [Install source (With Go Installed)](#install-source-with-go-installed)
		* [`go install ...`](#go-install)
		* [`go build ...` the development Version](#go-build--the-development-version)
	* [Install on Docker (With Docker Installed)](#install-on-docker-with-docker-installed)
* [Post Installation](#post-installation)
* [Usage](#usage)
	* [Basic](#basic)
	* [Filter Regex](#filter-regex)
	* [Match Regex](#match-regex)
* [Contributing](#contributing)
* [Licensing](#licensing)
* [Credits](#credits)
	* [Contributors](#contributors)
	* [Similar Projects](#similar-projects)

## Features

* Fetches URLs from multiple online passive sources to provide extensive results
* Supports `stdin` and `stdout` for easy integration in automated workflows
* Cross-Platform (Windows, Linux, and macOS)

## Installation

### Install release binaries (Without Go Installed)

Visit the [releases page](https://github.com/hueristiq/xurlfind3r/releases) and find the appropriate archive for your operating system and architecture. Download the archive from your browser or copy its URL and retrieve it with `wget` or `curl`:

* ...with `wget`:

	```bash
	wget https://github.com/hueristiq/xurlfind3r/releases/download/v<version>/xurlfind3r-<version>-linux-amd64.tar.gz
	```

* ...or, with `curl`:

	```bash
	curl -OL https://github.com/hueristiq/xurlfind3r/releases/download/v<version>/xurlfind3r-<version>-linux-amd64.tar.gz
	```

...then, extract the binary:

```bash
tar xf xurlfind3r-<version>-linux-amd64.tar.gz
```

> [!TIP]
> The above steps, download and extract, can be combined into a single step with this onliner
> 
> ```bash
> curl -sL https://github.com/hueristiq/xurlfind3r/releases/download/v<version>/xurlfind3r-<version>-linux-amd64.tar.gz | tar -xzv
> ```

> [!NOTE]
> On Windows systems, you should be able to double-click the zip archive to extract the `xurlfind3r` executable.

...move the `xurlfind3r` binary to somewhere in your `PATH`. For example, on GNU/Linux and OS X systems:

```bash
sudo mv xurlfind3r /usr/local/bin/
```

> [!NOTE]
> Windows users can follow [How to: Add Tool Locations to the PATH Environment Variable](https://msdn.microsoft.com/en-us/library/office/ee537574(v=office.14).aspx) in order to add `xurlfind3r` to their `PATH`.


### Install source (With Go Installed)

Before you install from source, you need to make sure that Go is installed on your system. You can install Go by following the official instructions for your operating system. For this, we will assume that Go is already installed.

#### `go install ...`

```bash
go install -v github.com/hueristiq/xurlfind3r/cmd/xurlfind3r@latest
```

#### `go build ...` the development Version

* Clone the repository

	```bash
	git clone https://github.com/hueristiq/xurlfind3r.git 
	```

* Build the utility

	```bash
	cd xurlfind3r/cmd/xurlfind3r && \
	go build .
	```

* Move the `xurlfind3r` binary to somewhere in your `PATH`. For example, on GNU/Linux and OS X systems:

	```bash
	sudo mv xurlfind3r /usr/local/bin/
	```

	Windows users can follow [How to: Add Tool Locations to the PATH Environment Variable](https://msdn.microsoft.com/en-us/library/office/ee537574(v=office.14).aspx) in order to add `xurlfind3r` to their `PATH`.


> [!CAUTION]
> While the development version is a good way to take a peek at `xurlfind3r`'s latest features before they get released, be aware that it may have bugs. Officially released versions will generally be more stable.

### Install on Docker (With Docker Installed)

To install `xurlfind3r` on docker:

* Pull the docker image using:

    ```bash
    docker pull hueristiq/xurlfind3r:latest
    ```

* Run `xurlfind3r` using the image:

    ```bash
    docker run --rm hueristiq/xurlfind3r:latest -h
    ```

## Post Installation

`xurlfind3r` will work right after [installation](#installation). However, some sources require API keys to work. These keys can be added to a configuration file at `$HOME/.config/xurlfind3r/config.yaml`, created upon first run, or set as environment variables.

Example of environment variables for API keys:

```bash
XURLFIND3R_KEYS_BEVIGIL=your_bevigil_key
XURLFIND3R_KEYS_ONTELX=your_intelx_key
```

## Usage

To start using `xurlfind3r`, open your terminal and run the following command for a list of options:

```bash
xurlfind3r -h
```

Here's what the help message looks like:

```

                 _  __ _           _ _____
__  ___   _ _ __| |/ _(_)_ __   __| |___ / _ __
\ \/ / | | | '__| | |_| | '_ \ / _` | |_ \| '__|
 >  <| |_| | |  | |  _| | | | | (_| |___) | |
/_/\_\\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_|
                                          v0.4.0

USAGE:
 xurlfind3r [OPTIONS]

CONFIGURATION:
 -c, --configuration string          configuration file (default: $HOME/.cfg/xurlfind3r/config.yaml)

INPUT:
 -d, --domain string[]               target domain
 -l, --list string                   target domains' list file path

TIP: For multiple input domains use comma(,) separated value with `-d`,
     specify multiple `-d`, load from file with `-l` or load from stdin.

SCOPE:
     --include-subdomains bool       match subdomain's URLs

SOURCES:
     --sources bool                  list supported sources
 -u, --use-sources string[]          comma(,) separated sources to use
 -e, --exclude-sources string[]      comma(,) separated sources to exclude

FILTER & MATCH:
 -f, --filter string                 regex to filter URLs
 -m, --match string                  regex to match URLs

OUTPUT:
     --no-color bool                 disable colored output
 -o, --output string                 output URLs file path
 -O, --output-directory string       output URLs directory path
 -s, --silent bool                   display output subdomains only
 -v, --verbose bool                  display verbose output

pflag: help requested
```

### Examples

#### Basic

```bash
xurlfind3r -d hackerone.com --include-subdomains
```

#### Filter Regex

```bash
# filter images
xurlfind3r -d hackerone.com --include-subdomains -f '`^https?://[^/]*?/.*\.(jpg|jpeg|png|gif|bmp)(\?[^\s]*)?$`'
```

#### Match Regex

```bash
# match js URLs
xurlfind3r -d hackerone.com --include-subdomains -m '^https?://[^/]*?/.*\.js(\?[^\s]*)?$'
```

## Contributing

We welcome contributions! Feel free to submit [Pull Requests](https://github.com/hueristiq/xurlfind3r/pulls) or report [Issues](https://github.com/hueristiq/xurlfind3r/issues). For more details, check out the [contribution guidelines](https://github.com/hueristiq/xurlfind3r/blob/master/CONTRIBUTING.md).

## Licensing

This utility is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/xurlfind3r/blob/master/LICENSE).

## Credits

### Contributors

A huge thanks to all the contributors who have helped make `xurlfind3r` what it is today!

[![contributors](https://contrib.rocks/image?repo=hueristiq/xurlfind3r&max=500)](https://github.com/hueristiq/xurlfind3r/graphs/contributors)

### Similar Projects

If you're interested in more utilities like this, check out:

[gau](https://github.com/lc/gau) ◇ [waybackurls](https://github.com/tomnomnom/waybackurls) ◇ [waymore](https://github.com/xnl-h4ck3r/waymore)