# {{.Name}}

{{pkgdoc}}

# {{toc 3}}

# Install

## go

{{template "go/install" .}}

# Cli

## Help

#### $ {{exec "go" "run" "main.go" "-help" | color "sh"}}

# Example

## Filter versions

#### $ {{exec "go" "run" "main.go" "-c" "1.x" "1.0.4 1.1.1 1.2.2 2.3.4" | color "sh"}}

## Use stdin

#### $ {{shell "echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -c 2.x" | color "sh"}}

## Sort version

#### $ {{shell "echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s" | color "sh"}}

## Sort version descending, take only the first

#### $ {{shell "echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -f" | color "sh"}}

## Sort version descending, take only the last

#### $ {{shell "echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -l" | color "sh"}}

## Sort version descending, output to json

#### $ {{shell "echo '1.0.4 1.1.1 1.2.2 2.3.4' | go run main.go -s -d -j" | color "sh"}}

## Select only non version

#### $ {{shell "echo "0.0.4 1.2.3 tomate 0.3.2" | go run main.go -invalid" | color "sh"}}
