# xurlfind3r

![made with go](https://img.shields.io/badge/made%20with-Go-0000FF.svg) [![release](https://img.shields.io/github/release/hueristiq/xurlfind3r?style=flat&color=0000FF)](https://github.com/hueristiq/xurlfind3r/releases) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=0000FF)](https://github.com/hueristiq/xurlfind3r/blob/master/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-0000FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/xurlfind3r.svg?style=flat&color=0000FF)](https://github.com/hueristiq/xurlfind3r/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/xurlfind3r.svg?style=flat&color=0000FF)](https://github.com/hueristiq/xurlfind3r/issues?q=is:issue+is:closed) [![contribution](https://img.shields.io/badge/contributions-welcome-0000FF.svg)](https://github.com/hueristiq/xurlfind3r/blob/master/CONTRIBUTING.md)

`xurlfind3r` is a command-line interface (CLI) utility to find domain's known URLs passively from **[AlienVault's Open Threat Exchange](https://otx.alienvault.com/)**, **[Common Crawl](https://commoncrawl.org/)**, **[Github](https://github.com)**, **[Intelligence X](https://intelx.io)**, **[URLScan](https://urlscan.io/)**, and the **[Wayback Machine](https://archive.org/web/)**.

## Resource

* [Features](#features)
* [Installation](#installation)
	* [Install release binaries](#install-release-binaries)
	* [Install source](#install-sources)
		* [`go install ...`](#go-install)
		* [`go build ...` the development Version](#go-build--the-development-version)
* [Post Installation](#post-installation)
* [Usage](#usage)
* [Contribution](#contribution)
* [Licensing](#licensing)

## Features

* Fetches known URLs from **[AlienVault's OTX](https://otx.alienvault.com/)**, **[Common Crawl](https://commoncrawl.org/)**, **[URLScan](https://urlscan.io/)**, **[Github](https://github.com)**, **[Intelligence X](https://intelx.io)** and the **[Wayback Machine](https://archive.org/web/)**.
* Parse URLs from `robots.txt` snapshots on the Wayback Machine.
* Parse URLs from webpages snapshots on the Wayback Machine.

## Installation

### Install release binaries

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

> **TIP:** The above steps, download and extract, can be combined into a single step with this onliner
> 
> ```bash
> curl -sL https://github.com/hueristiq/xurlfind3r/releases/download/v<version>/xurlfind3r-<version>-linux-amd64.tar.gz | tar -xzv
> ```

**NOTE:** On Windows systems, you should be able to double-click the zip archive to extract the `xurlfind3r` executable.

...move the `xurlfind3r` binary to somewhere in your `PATH`. For example, on GNU/Linux and OS X systems:

```bash
sudo mv xurlfind3r /usr/local/bin/
```

**NOTE:** Windows users can follow [How to: Add Tool Locations to the PATH Environment Variable](https://msdn.microsoft.com/en-us/library/office/ee537574(v=office.14).aspx) in order to add `xurlfind3r` to their `PATH`.

### Install source

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

	**NOTE:** Windows users can follow [How to: Add Tool Locations to the PATH Environment Variable](https://msdn.microsoft.com/en-us/library/office/ee537574(v=office.14).aspx) in order to add `xurlfind3r` to their `PATH`.


**NOTE:** While the development version is a good way to take a peek at `xurlfind3r`'s latest features before they get released, be aware that it may have bugs. Officially released versions will generally be more stable.

## Post Installation

`xurlfind3r` will work right after [installation](#installation). However, **[Github](https://github.com)** and **[Intelligence X](https://intelx.io)** require API keys to work. The API keys are stored in the `$HOME/.hueristiq/xurlfind3r/config.yaml` file - created upon first run - and uses the YAML format.

Example:

```yaml
version: 0.1.0
sources:
    - commoncrawl
    - github
    - intelx
    - otx
    - urlscan
    - wayback
keys:
    github:
        - d23a554bbc1aabb208c9acfbd2dd41ce7fc9db39
        - asdsd54bbc1aabb208c9acfbd2dd41ce7fc9db39
    intelx:
        - 2.intelx.io:00000000-0000-0000-0000-000000000000
```

## Usage

To display help message for `xurlfind3r` use the `-h` flag:

```bash
xurlfind3r -h
```

help message:

```
                 _  __ _           _ _____      
__  ___   _ _ __| |/ _(_)_ __   __| |___ / _ __ 
\ \/ / | | | '__| | |_| | '_ \ / _` | |_ \| '__|
 >  <| |_| | |  | |  _| | | | | (_| |___) | |
/_/\_\\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_| v0.1.0

A CLI utility to find domain's known URLs.

USAGE:
  xurlfind3r [OPTIONS]

TARGET:
  -d, --domain string             target domain
      --include-subdomains bool   include domain's subdomains

SOURCES:
      --list-sources bool         list available sources
 -s   --sources strings           comma(,) separated sources to use (default: commoncrawl,github,intelx,otx,urlscan,wayback)

CONFIGURATION:
      --skip-wayback-robots bool  skip parsing wayback robots.txt snapshots
      --skip-wayback-source bool  skip parsing wayback source code snapshots

OUTPUT:
  -m, --monochrome                no colored output mode
  -o, --output string             output file to write found URLs
  -v, --verbosity                 debug, info, warning, error, fatal or silent (default: info)
```

## Contribution

[Issues](https://github.com/hueristiq/xurlfind3r/issues) and [Pull Requests](https://github.com/hueristiq/xurlfind3r/pulls) are welcome! Check out the [contribution guidelines](./CONTRIBUTING.md).

## Licensing

This utility is distributed under the [MIT license](./LICENSE).