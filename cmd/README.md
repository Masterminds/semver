# cmd

Package cmd implement a cli tool to manipulate Versions.


# TOC
- [Install](#install)
  - [go](#go)
- [Cli](#cli)
  - [Help](#help)
    - [$ go run main.go -help](#-go-run-maingo--help)
- [Example](#example)
  - [Filter versions](#filter-versions)
    - [$ go run main.go -c 1.x 1.0.4 1.1.1 1.2.2 2.3.4](#-go-run-maingo--c-1x-104-111-122-234)
  - [Use stdin](#use-stdin)
    - [$ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -c 2.x](#-echo-'104-111-122-234'-|-go-run-maingo--c-2x)
  - [Sort version](#sort-version)
    - [$ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s](#-echo-'104-111-122-234'-|-go-run-maingo--s)
  - [Sort version descending, take only the first](#sort-version-descending,-take-only-the-first)
    - [$ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -f](#-echo-'104-111-122-234'-|-go-run-maingo--s--d--f)
  - [Sort version descending, take only the last](#sort-version-descending,-take-only-the-last)
    - [$ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -l](#-echo-'104-111-122-234'-|-go-run-maingo--s--d--l)
  - [Sort version descending, output to json](#sort-version-descending,-output-to-json)
    - [$ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -j](#-echo-'104-111-122-234'-|-go-run-maingo--s--d--j)

# Install

## go

```sh
go get github.com/semver/cmd
```

# Cli

## Help

#### $ go run main.go -help
```sh
semver - 0.0.0

Usage

	-filter|-c  string  Filter versions matching given semver constraint
	-invalid    bool    Show only invalid versions

	-sort|-s    bool    Sort input versions
	-desc|-d    bool    Sort versions descending

	-first|-f   bool    Only first version
	-last|-l    bool    Only last version

	-json|-j    bool    JSON output

	-version    bool    Show version

Example

	semver -c 1.x 0.0.4 1.2.3
	exho "0.0.4 1.2.3" | semver -j
	exho "0.0.4 1.2.3" | semver -s
	exho "0.0.4 1.2.3" | semver -s -d -j -f
	exho "0.0.4 1.2.3 tomate" | semver -invalid
```

# Example

## Filter versions

#### $ go run main.go -c 1.x 1.0.4 1.1.1 1.2.2 2.3.4
```sh
- 1.0.4
- 1.1.1
- 1.2.2
```

## Use stdin

#### $ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -c 2.x
```sh
- 2.3.4
```

## Sort version

#### $ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s
```sh
- 1.0.4
- 1.1.1
- 1.2.2
- 2.3.4
```

## Sort version descending, take only the first

#### $ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -f
```sh
- 2.3.4
```

## Sort version descending, take only the last

#### $ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -l
```sh
- 1.0.4
```

## Sort version descending, output to json

#### $ echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -j
```sh
2.3.41.2.21.1.11.0.4
```
