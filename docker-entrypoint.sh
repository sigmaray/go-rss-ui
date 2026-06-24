#!/bin/sh
set -e

./go-rss-ui migrate
exec ./go-rss-ui "$@"
