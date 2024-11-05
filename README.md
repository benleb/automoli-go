# [![automoli](https://socialify.git.ci/benleb/automoli-go/image?description=1&font=KoHo&forks=1&language=1&logo=https%3A%2F%2Femojipedia-us.s3.dualstack.us-west-1.amazonaws.com%2Fthumbs%2F240%2Fapple%2F237%2Felectric-light-bulb_1f4a1.png&owner=1&pulls=1&stargazers=1&theme=Light)](https://github.com/benleb/automoli-go)

<!-- # AutoMoLi - **Auto**matic **Mo**tion **Li**ghts -->

[![Go Report Card](https://goreportcard.com/badge/github.com/benleb/automoli-go)](https://goreportcard.com/report/github.com/benleb/automoli-go) [![Go Reference](https://pkg.go.dev/badge/github.com/benleb/automoli-go.svg)](https://pkg.go.dev/github.com/benleb/automoli-go) [![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/benleb/automoli-go/build.yml
)](https://github.com/benleb/automoli-go/actions/workflows/build.yml) [![No Maintenance Intended](http://unmaintained.tech/badge.svg)](http://unmaintained.tech/)

Fully *automatic light management* based on motion, daytime, brightness and even humidity 💦 🚿  

🕓 multiple **daytimes** to define different scenes for morning, noon, ...  
💡 supports **Hue** (for Hue Rooms/Groups) & **Home Assistant** scenes  
🔌 switches **lights** and **plugs** (with lights)  
☀️ supports **illumination sensors** to switch the light just if needed  
💦 supports **humidity sensors** as blocker (the "*shower case*")  
🔒 **locks** the light if the light was manually turned on  
<!-- not yet implemented in the go version: -->
<!-- 🔍 **automatic** discovery of **lights** and **sensors**   -->
<!-- ⛰️ **stable** and **tested** by many people with different homes   -->  

*- successor of the famous original [ad-AutoMoLi](https://github.com/benleb/ad-automoli) (written in Python as [AppDaemon](https://github.com/AppDaemon/appdaemon) plugin/app) -*

## install

via [go install](https://go.dev/ref/mod#go-install)

```bash
go install github.com/benleb/automoli-go@latest
```

## run

see the [example config](automoli.yaml) for a multi-room configuration with different daytimes and sensors and settings.

```bash
# run
automoli-go run --config ~/automoli.yaml

# more options
automoli-go --help
```

### systemd service example

this is an **example** how the [systemd service file](automoli.service) can be used for running AutoMoLi as a service. the user, group and repo/config directory can be changed to your needs.

```bash
# clone repo
git clone https://github.com/benleb/automoli-go ~/automoli

# create a new user and group for automoli
useradd --system --home-dir /etc/automoli --user-group automoli

# create config directory and set permissions
mkdir /etc/automoli && chown automoli:automoli /etc/automoli

# link or copy the systemd service file
ln -s ~/automoli/automoli.service /etc/systemd/system/automoli.service
```

## build

### single target

```bash
# build for current platform
goreleaser build --clean --snapshot --single-target

# build for specific platform
GOOS="linux" GOARCH="amd64" GOAMD64="v3" goreleaser build --clean --snapshot --single-target
```

### docker image

with [ko](https://ko.build)

```bash
# build image and push to registry
KO_DOCKER_REPO=your.registry.io:5000 ko build --verbose --base-import-paths --tags dev
```

## development

### lint

with [golangci-lint](https://golangci-lint.run)

```bash
# run all linters
golangci-lint run --verbose --enable-all --fix --max-issues-per-linter 0 --max-same-issues 0
```

### tests

```bash
# run tests with coverage
go test -cover ./...
```

### release/tag

vith [goreleaser](https://goreleaser.com) triggered by a git tag

```bash
# create a new annotated tag
git tag -a "vX.Y.Z" -m "short release description vX.Y.Z"
# push tag to trigger the release workflow
git push --follow-tags
```
