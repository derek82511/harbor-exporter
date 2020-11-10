#!/bin/bash

# linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/linux-amd64/harbor-exporter

# darwin amd64
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/darwin-amd64/harbor-exporter
