#!/bin/zsh
# Compatibility entry point for the standalone image publisher.
set -eu
root=${0:A:h:h:h}
case ${1:-all} in
    all) exec python3 "$root/scripts/images.py" build login-go chat-go game-go gamesniff panel ;;
    login|chat|game) exec python3 "$root/scripts/images.py" build "$1-go" ;;
    gamesniff|panel) exec python3 "$root/scripts/images.py" build "$1" ;;
    *) print -u2 'usage: build-images.sh [all|login|chat|game|gamesniff|panel]'; exit 2 ;;
esac
