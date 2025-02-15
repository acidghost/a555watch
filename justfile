program := 'a555watch'

goos := if os() == 'macos' { 'darwin' } else { os() }
goarch := if arch() == 'aarch64' { 'arm64' } else { arch() }

alias b := build
alias r := run-local

build-all: (build 'darwin' 'arm64') (build 'linux' 'arm64') (build 'linux' 'amd64')

build os=goos arch=goarch: build-dir
    GOOS={{os}} GOARCH={{arch}} go build -o build/{{program}}-{{os}}-{{arch}}

build-dir:
    mkdir -p build

run-local *args: build
    ./build/{{program}}-{{goos}}-{{goarch}} {{args}}

vendor:
    go mod tidy
    go mod vendor

update-readme: build
    cp -ap ./build/{{program}}-{{goos}}-{{goarch}} ./build/{{program}}
    uv tool run --from=cogapp@3.4.1 cog -r README.md

lint:
    golangci-lint run

clean:
    rm -rf build
