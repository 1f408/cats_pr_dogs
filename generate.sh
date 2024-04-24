#!/bin/sh
(cd conf; GOOS= GOARCH= go run ./gen)
