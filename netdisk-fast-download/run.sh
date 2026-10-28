#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR" || exit 1
exec "$DIR/jre/bin/java" -Xmx512M -Dfile.encoding=utf-8 -jar "$DIR/netdisk-fast-download.jar" "$@"
