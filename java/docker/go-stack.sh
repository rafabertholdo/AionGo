#!/bin/zsh
# Restarts the Go game server (al19-game-go) behind the sniffer (al19-game); with "login", the login server first.
# Kills the Aion client, whose relays point at the old containers.
source "${0:A:h}/image-config.zsh"
ip() { container list --format json | python3 -c "import json,sys; print([c['networks'][0]['address'].split('/')[0] for c in json.load(sys.stdin) if c['configuration']['id']=='$1'][0])"; }
AP=$(/bin/ps ax -o pid,command | /usr/bin/grep '[Z]:.*bin32.aion\.bin' | awk '{print $1}'); [ -n "$AP" ] && kill $AP
for n in al19-game al19-game-go al19-game-java; do container delete --force $n >/dev/null 2>&1; done
DB=$(ip al19-db); CS=$(ip al19-chat)
if [ "$1" = login ]; then
  container delete --force al19-login >/dev/null 2>&1
  container run --detach --name al19-login -e AION_DB=$DB $login_go_image >/dev/null; sleep 3
fi
LS=$(ip al19-login)
container run --detach --name al19-game-go -e AION_DB=$DB -e AION_LS=$LS -e AION_CS=$CS -e HOST_NAME=127.0.0.1 -e AION_DEBUG=1 $game_go_image >/dev/null; sleep 3
container run --detach --name al19-game -e HOST_NAME=127.0.0.1 $gamesniff_image -target $(ip al19-game-go):7777 >/dev/null; sleep 2
container logs al19-game-go | grep -v DEBUG | tail -3
