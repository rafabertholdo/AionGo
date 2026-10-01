#!/bin/zsh
source "${0:A:h}/image-config.zsh"
# Starts the old (morning) Aion stack with a decrypting sniffer in front of its game server.
# The user connects in ReRun's Address mode to the printed login address. Deletes the al19-* game/sniffer containers first.
ip() { container list --format json | python3 -c "import json,sys; print([c['networks'][0]['address'].split('/')[0] for c in json.load(sys.stdin) if c['configuration']['id']=='$1'][0])"; }
AP=$(/bin/ps ax -o pid,command | /usr/bin/grep '[Z]:.*bin32.aion\.bin' | awk '{print $1}'); [ -n "$AP" ] && kill $AP
for n in al19-game al19-game-go al19-game-java aion-sniff aion-gs; do container delete --force $n >/dev/null 2>&1; done
(cd /Volumes/acasis/games/aion/servers/AionLightning1.9 && ./aion-servers.sh up 2>&1 | tail -1)
container delete --force aion-gs >/dev/null 2>&1
# Containers get consecutive addresses, wrapping at .254: a probe shows the next one, and a wrong guess is retried.
for try in 1 2 3 4 5 6; do
  container delete --force aion-sniff >/dev/null 2>&1; container delete --force aion-gs >/dev/null 2>&1
  container run --detach --name probe docker.io/library/golang:1.25 sleep 20 >/dev/null 2>&1
  P=$(ip probe | cut -d. -f4); container delete --force probe >/dev/null 2>&1
  SN=192.168.64.$((P+1)); GS=192.168.64.$((P+2))
  container run --detach --name aion-sniff $gamesniff_image -target $GS:7777 >/dev/null
  DB=$(ip aion-db)
  container run -d --name aion-gs --arch amd64 -m 3G -e AION_DB=$DB -e AION_DB_USER=root -e AION_DB_PASSWORD=aion -e AION_LS=$(ip aion-ls) -e AION_CS=$(ip aion-cs) -e HOST_NAME=$SN rafabertholdo/aion-gs:1.9 >/dev/null
  [ "$(ip aion-sniff)" = "$SN" ] && [ "$(ip aion-gs)" = "$GS" ] && break
done
until container logs aion-gs 2>&1 | /usr/bin/grep -q "Total Boot Time"; do sleep 3; done
echo "sniffer=$(ip aion-sniff) gs=$(ip aion-gs) LOGIN=$(ip aion-ls)"
