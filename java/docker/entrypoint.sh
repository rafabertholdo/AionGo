#!/bin/sh
# Starts one Aion Lightning server with its config filled in from the
# environment. The config is copied from config.template on every start, so a
# container restarted with new addresses never keeps the old ones.
#
# AION_DB, AION_DB_USER, AION_DB_PASSWORD   database (login and game server)
# AION_LS, AION_LS_PASSWORD, AION_GSID      login server (game server)
# AION_CS                                   chat server (game server)
# SERVER_NAME, SERVER_CC                    server list entry (game server)
# HOST_NAME                                 address the client is sent to connect to;
#                                           the game server defaults to its own hostname
set -eu
cd /opt/aion

: "${AION_DB_USER:=root}" "${AION_DB_PASSWORD:=aion}" "${AION_LS_PASSWORD:=aion}"
: "${AION_GSID:=1}" "${SERVER_NAME:=Siel}" "${SERVER_CC:=1}" "${HOST_NAME:=$(hostname)}"
export AION_DB_USER AION_DB_PASSWORD AION_LS_PASSWORD AION_GSID SERVER_NAME SERVER_CC HOST_NAME

rm -rf config
cp -R config.template config
# Java's game state stays separate from the Go server when both run together.
if [ "$AION_MAIN" = com.aionemu.gameserver.GameServer ]; then
    : "${AION_GS_DB:=au_server_gs_java}"
    case "$AION_GS_DB" in *[!a-zA-Z0-9_]*) echo "Invalid AION_GS_DB" >&2; exit 1;; esac
    sed -i "s/au_server_gs/$AION_GS_DB/g" config/network/database.properties
fi
# Longest names first: AION_DB is a prefix of AION_DB_USER and AION_DB_PASSWORD.
for name in AION_DB_PASSWORD AION_DB_USER AION_DB AION_LS_PASSWORD AION_LS AION_CS AION_GSID SERVER_NAME SERVER_CC HOST_NAME; do
    value=$(printenv "$name" || true)
    [ -n "$value" ] || continue
    grep -rl "$name" config | while read -r file; do
        sed -i "s|$name|$value|g" "$file"
    done
done

# shellcheck disable=SC2086 # JAVA_OPTS holds several options.
exec java $JAVA_OPTS -cp "libs/*:$AION_JAR" "$AION_MAIN"
