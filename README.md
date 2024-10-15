# AutoMoLi 💡

[![Go Report Card](https://goreportcard.com/badge/github.com/benleb/automoli)](https://goreportcard.com/report/github.com/benleb/automoli) [![Go Reference](https://pkg.go.dev/badge/github.com/benleb/automoli.svg)](https://pkg.go.dev/github.com/benleb/automoli) [![GitHub Workflow Status](https://img.shields.io/github/workflow/status/benleb/automoli/build)](https://github.com/benleb/automoli/actions/workflows/build.yml) [![No Maintenance Intended](http://unmaintained.tech/badge.svg)](http://unmaintained.tech/)

## build

### single target

```bash
# adjust variables to your needs
export GOOS="linux" GOARCH="amd64" GOAMD64="v3"
goreleaser build --clean --snapshot --single-target
```

### ko docker image

```bash
# add your docker registry
export KO_DOCKER_REPO=your.registry.io:5000
ko build --verbose --base-import-paths --tags dev
```

## linting

`golangci-lint run --verbose --enable-all --fix --max-issues-per-linter 0 --max-same-issues 0`

## systemd service example

### clone repository

`git clone <https://github.com/benleb/automoli> ~/automoli`

### create user and config directory

`useradd --system --home-dir /etc/automoli --user-group automoli`  
`mkdir /etc/automoli && chown automoli:automoli /etc/automoli`

### link or copy the systemd service file

`ln -s ~/automoli/automoli.service /etc/systemd/system/automoli.service`

### tests (todo)

`go test -cover ./...`

