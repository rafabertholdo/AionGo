#!/bin/zsh
# Builds, publishes and runs Aion Lightning 1.9 (Java 21, MariaDB 11, arm64) on
# Apple's `container`. ReRun's Aion tab does the same `up` with one button (Play
# with Server: This Mac) and relays 127.0.0.1 to the containers; it reuses a
# stack started here, since both use the same names and HOST_NAME.
#   docker/aion-servers.sh build      build the four images from this source tree
#   docker/aion-servers.sh push       publish them to Docker Hub, where ReRun pulls them
#   docker/aion-servers.sh up         start database, login, chat and game servers
#   docker/aion-servers.sh down       stop and remove them (the database volume is kept)
#   docker/aion-servers.sh logs game  follow a server's log (db, login, chat, game)
#   docker/aion-servers.sh sql        open a MariaDB shell
set -e
source "${0:A:h}/image-config.zsh"
cd "$aiongo_root/java"
case ${AION_IMPLEMENTATION:-go} in
    go) login_image=$login_go_image; chat_image=$chat_go_image; game_image=$game_go_image ;;
    java21) login_image=$login_java21_image; chat_image=$chat_java21_image; game_image=$game_java21_image ;;
    *) echo "AION_IMPLEMENTATION must be go or java21" >&2; exit 2 ;;
esac

# Containers are always created afresh: their addresses change on every start,
# and each server writes the others' addresses into its config when it starts.
# No ports are published: container 0.4's forwarder accepts on the Mac but never
# reaches the container, so ReRun relays 127.0.0.1 to the containers instead.
run() {
    local name=$1; shift
    container rm -f $name >/dev/null 2>&1 || true
    container run -d --name $name "$@" >/dev/null
}

ip() {
    container inspect $1 | python3 -c 'import sys,json; print(json.load(sys.stdin)[0]["networks"][0]["address"].split("/")[0])'
}

# Waits for `$2` in container $1's log, or stops with its log if the server
# fails first. A failed startup can leave the JVM (and so the container) running
# on its other threads, so its log is checked as well as its state.
wait_log() {
    until container logs $1 2>&1 | tail -50 | grep -q "$2"; do
        if ! container ls | grep -q "^$1 " || container logs $1 2>&1 | grep -q 'Exception in thread "main"'; then
            container logs $1 2>&1 | tail -20
            echo "$1 stopped before it was ready." >&2
            exit 1
        fi
        sleep 2
    done
}

case $1 in
build)
    python3 "$aiongo_root/scripts/images.py" build
    ;;
push)
    python3 "$aiongo_root/scripts/images.py" push
    ;;
up)
    container system start >/dev/null
    container volume ls | grep -q al19-db-data || container volume create al19-db-data >/dev/null
    run al19-db -e MARIADB_ROOT_PASSWORD=aion -v al19-db-data:/var/lib/mysql $db_image
    echo "Waiting for the database…"
    until container exec al19-db mariadb-admin -uroot -paion -h127.0.0.1 --protocol=tcp ping >/dev/null 2>&1; do sleep 2; done
    db=$(ip al19-db)
    run al19-login -e AION_DB=$db $login_image
    run al19-chat $chat_image
    echo "Waiting for the login server…"
    wait_log al19-login "Total Boot Time"
    # Clients connect at 127.0.0.1: ReRun relays it here, and cloudflared on a friend's Mac.
    run al19-game -m 3G -e AION_DB=$db -e AION_LS=$(ip al19-login) -e AION_CS=$(ip al19-chat) -e HOST_NAME=127.0.0.1 $game_image
    echo "Waiting for the game server (about a minute)…"
    wait_log al19-game "Total Boot Time"
    echo "Ready. Play from ReRun's Aion tab with Server: This Mac; it relays 127.0.0.1 here."
    echo "Any new account name is created on first login."
    ;;
down)
    container rm -f al19-game al19-chat al19-login al19-db >/dev/null 2>&1 || true
    ;;
logs)
    container logs -f al19-$2
    ;;
sql)
    container exec -it al19-db mariadb -uroot -paion
    ;;
*)
    echo "usage: $0 build | push | up | down | logs db|login|chat|game | sql"; exit 1
    ;;
esac
