program := 'a555watch'

version := 'SNAPSHOT-'+`git describe --tags --always --dirty`
commit_sha := `git rev-parse HEAD`
build_time := `date -u '+%Y-%m-%d_%H:%M:%S'`

ldflags := '-s -w -X main.buildVersion='+version \
        +' -X main.buildCommit='+commit_sha \
        +' -X main.buildDate='+build_time

goos := if os() == 'macos' { 'darwin' } else { os() }
goarch := if arch() == 'aarch64' { 'arm64' } else { arch() }

alias b := build
alias r := run-local

build-all: (build 'darwin' 'arm64') (build 'linux' 'arm64') (build 'linux' 'amd64')

build os=goos arch=goarch: build-dir
    CGO_ENABLED=0 GOOS={{os}} GOARCH={{arch}} \
        go build \
            -ldflags '{{ldflags}}' \
            -o build/{{program}}-{{os}}-{{arch}}

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
