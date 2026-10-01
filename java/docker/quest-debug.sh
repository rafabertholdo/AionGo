#!/bin/zsh
# Java-vs-Go game server debug environment. Read go/QUEST_DEBUG.md.
#
# BOTH game servers run and are registered at once, so the client's server list has two entries:
#   entry 1 = id 1 = GO    127.0.0.1:7777          ReRun relay -> sniffer al19-game           -> al19-game-go
#   entry 2 = id 2 = JAVA  <sniffer2 ip>:7777      dialed DIRECTLY (no relay): al19-game-java-sniff -> al19-game-java
# The Java server registers the address of its own sniffer, so the client reaches it without ReRun's relay.
# Container IPs change on every (re)create: this script reads them at run time.
#
#   quest-debug.sh up                       ensure db, chat, login (2 game server rows), Go+Java servers, 2 sniffers;
#                                           prints what to press in ReRun. Idempotent; only recreates what is stale.
#   quest-debug.sh status [charname]        which list entry is Go/Java, ips, registrations, sniffer logs, rows
#   quest-debug.sh mark <label>             write a labelled marker into BOTH sniffer logs
#   quest-debug.sh log java|go [label]      save that sniffer's log (from the last marker <label>) and print the path
#   quest-debug.sh bot go|java [aionbot flags]   scripted player through that entry's real path (dumps SM_SERVER_LIST)
#   quest-debug.sh reset-quest <char> <questId> [itemId...]   delete the quest row (and those items); with RESET_TO=START put the row back as START/0 instead (the campaign quests 1001-1005/2001-2005 start on level-up, so their giver only offers them from that row)
#   quest-debug.sh prep <char> <questId>    put the character in the quest's start state on Go AND Java (each schema that exists): snapshot,
#                                           quest row (START/0 for level-up quests, else none), prerequisites COMPLETE, level >= the quest's,
#                                           quest items removed, teleported next to the start npc. Log the character out of both servers first.
#   quest-debug.sh compare <labelJava> <labelGo> [questId] [pktdiff flags]   pktdiff -dialogs on the two labelled log slices (refreshed from
#                                           the sniffers when they run, else the newest saved ones) + the Java handler table for the quest
#   quest-debug.sh snapshot <char> [tag]    save the character's rows to a .sql file
#   quest-debug.sh restore  <char> [tag]    put them back (character must be logged out of both servers)
#   quest-debug.sh sync-java                re-copy the Java schema au_server_gs_java from the Go one (au_server_gs), same start state
#   quest-debug.sh down                     remove the Java server + its sniffer + list entry 2: the Go-only stack
#
# Java image: JAVA_GS=old (default; rafabertholdo/aion-gs:1.9, decompiled from the real 1.0.1 jars, amd64
# under Rosetta) or JAVA_GS=source (rafabertholdo/aiongo:1.9-game-java21-v0.1.0, built from AL-Game, arm64); remembered.
# It never removes al19-db or its volume, and only touches al19-game, al19-game-go, al19-game-java,
# al19-game-java-sniff, and al19-login (only when its game server table does not match the loaded one).
setopt no_unset pipefail
src=${0:A:h:h}
self=${0:A}
source "${0:A:h}/image-config.zsh"
qd=${QD_DIR:-$src/docker/.quest-debug}
mkdir -p $qd/logs $qd/snapshots
JDB=au_server_gs_java
DB=${GS_DB:-au_server_gs}   # GS_DB=au_server_gs_java to snapshot/restore/reset-quest/status the Java server's copy
goapp=${AION_GO_DIR:-$src/../go}   # the Go port: scripts/quest-java-table.py, cmd/pktdiff
GO=al19-game-go JAVA=al19-game-java SNIFF=al19-game SNIFFJ=al19-game-java-sniff

ip() { container inspect $1 2>/dev/null | python3 -c 'import sys,json; print(json.load(sys.stdin)[0]["networks"][0]["address"].split("/")[0])' 2>/dev/null; }
running() { container ls 2>/dev/null | awk -v n=$1 '$1==n && $0 ~ /running/ {f=1} END{exit !f}'; }
# Value of environment variable $2 (or the first argument if $2 is "@args") in container $1.
cenv() { container inspect $1 2>/dev/null | python3 -c '
import sys,json
p=json.load(sys.stdin)[0]["configuration"]["initProcess"]
k=sys.argv[1]
if k=="@args": print(" ".join(p["arguments"]))
else: print(([e.split("=",1)[1] for e in p["environment"] if e.startswith(k+"=")] or [""])[-1])' $2 2>/dev/null; }
sql() { container exec -i al19-db mariadb -uroot -paion -N "$@"; }
die() { echo "quest-debug: $*" >&2; exit 1; }
javaimg() { [[ ${JAVA_GS:-$(cat $qd/java-image 2>/dev/null || echo old)} == source ]] && echo source || echo old; }
# The image the Java container was made from: old or source.
javaimg_of() { container inspect $JAVA 2>/dev/null | /usr/bin/grep -q 'aiongo:1.9-game-java21-v0.1.0' && echo source || echo old; }
wait_log() { # container, text, seconds
    local i; for i in {1..$(( $3 / 2 ))}; do
        container logs $1 2>&1 | /usr/bin/grep -q "$2" && return 0
        container logs $1 2>&1 | /usr/bin/grep -q 'Critical Error\|Exception in thread "main"' && { container logs $1 2>&1 | tail -15; die "$1 failed to start"; }
        running $1 || { container logs $1 2>&1 | tail -15; die "$1 stopped before it was ready"; }
        sleep 2
    done; container logs $1 2>&1 | tail -15; die "$1 not ready after $3 s (waiting for '$2')"
}
recreate() { local n=$1; shift; container delete --force $n >/dev/null 2>&1
    local i; for i in {1..15}; do container ls -a | awk -v n=$n '$1==n{f=1} END{exit f}' && break; sleep 1; done
    container run --detach --name $n "$@" >/dev/null || die "cannot start $n"; }
player() { [[ $1 =~ '^[A-Za-z0-9]+$' ]] || die "bad character name '$1'"; local id; id=$(sql -e "select id from $DB.players where name='$1'"); [[ -n $id ]] || die "no character $1"; echo $id; }
gs_tables() { sql -e "select table_name from information_schema.columns where table_schema='$DB' and column_name='player_id'"; }


do_snapshot() { # char tag   (schema $DB)
    local id f; id=$(player $1) || exit 1; f=$qd/snapshots/$1-$2.sql
    { echo "USE $DB;"; echo "SET FOREIGN_KEY_CHECKS=0;"
      dump() { container exec al19-db mariadb-dump -uroot -paion --no-create-info --replace --skip-add-locks --skip-disable-keys --skip-comments --where="$2" $DB $1; }
      dump players "id=$id"; dump inventory "itemOwner=$id"
      for t in $(gs_tables); do dump $t "player_id=$id"; done
      echo "SET FOREIGN_KEY_CHECKS=1;"; } > $f || die "dump failed"
    echo "saved $f ($(/usr/bin/grep -c '^REPLACE' $f) statements)"
}

# prep <char> <questId>: the quest's start state, on every game schema that exists (Go au_server_gs, Java $JDB).
do_prep() {
    local name=$1 qid=$2 script=$goapp/scripts/quest-java-table.py d id online race
    [[ -f $script ]] || die "$script not found (set AION_GO_DIR to Apps/AionServer)"
    command -v python3 >/dev/null || die "python3 is needed to read the Java handler"
    local q_found q_minlevel q_minexp q_levelup q_prereqs q_items q_npc q_world q_x q_y q_z q_h q_race
    eval "$(AION_JAVA=$src/AL-Game python3 $script $qid --shell)" || die "cannot read quest $qid from the Java sources ($src/AL-Game)"
    [[ $q_found == 1 ]] || die "quest $qid is not in quest_data.xml"
    command -v container >/dev/null && running al19-db || die "al19-db is not running. Bring the database up with: container start al19-db  (or the whole stack: $self up, which also starts the Go and Java game servers)"
    echo "quest $qid: minlevel $q_minlevel (exp $q_minexp), $([[ $q_levelup == 1 ]] && echo 'starts on LEVEL-UP' || echo "starts at npc $q_npc"), race $q_race, prerequisites: ${q_prereqs:-none}, quest items: ${q_items:-none}"
    local dbs=(au_server_gs)
    if [[ -n $(sql -e "show databases like '$JDB'") ]]; then dbs+=($JDB)
    else echo "note: no Java schema $JDB yet; it is created by '$self up' (or sync-java). Prepping the Go schema only."; fi
    local p sqltext
    for d in $dbs; do
        DB=$d
        echo; echo "== $d ($([[ $d == $JDB ]] && echo Java || echo Go))"
        id=$(sql -e "select id from $DB.players where name='$name'")
        if [[ -z $id ]]; then echo "  no character $name in $d$([[ $d == $JDB ]] && echo " (create it on Go, then: $self sync-java)"); skipped"; continue; fi
        online=$(sql -e "select online from $DB.players where id=$id"); race=$(sql -e "select race from $DB.players where id=$id")
        [[ $online == 1 ]] && echo "  WARNING: $name is flagged online in $d: log out first, the server rewrites its rows on logout and undoes this"
        [[ $q_race != ALL && $q_race != $race ]] && echo "  WARNING: quest $qid is for $q_race, $name is $race: the giver will not offer it"
        do_snapshot $name prep-$qid-$([[ $d == $JDB ]] && echo java || echo go) | sed 's/^/  snapshot: /'
        sqltext="start transaction;"
        for p in ${=q_prereqs}; do
            sqltext+="insert into $DB.player_quests (player_id,quest_id,status,quest_vars,complete_count) values ($id,$p,'COMPLETE',0,1) on duplicate key update status='COMPLETE',complete_count=greatest(complete_count,1);"; done
        if [[ $q_levelup == 1 ]]; then   # nothing offers a level-up quest: it needs its START/0 row
            sqltext+="insert into $DB.player_quests (player_id,quest_id,status,quest_vars,complete_count) values ($id,$qid,'START',0,0) on duplicate key update status='START',quest_vars=0,complete_count=0;"
        else sqltext+="delete from $DB.player_quests where player_id=$id and quest_id=$qid;"; fi
        sqltext+="update $DB.players set exp=greatest(exp,$q_minexp) where id=$id;"
        [[ -n $q_items ]] && sqltext+="delete from $DB.inventory where itemOwner=$id and itemId in (${q_items// /,});"
        [[ $q_world != 0 ]] && sqltext+="update $DB.players set world_id=$q_world,x=$q_x+1.5,y=$q_y,z=$q_z,heading=$q_h where id=$id;"
        sqltext+="commit;"
        print -r -- "$sqltext" | sql || die "prep failed on $d (nothing changed: one transaction)"
        echo "  quest row: $(sql -e "select concat(status,'/',quest_vars) from $DB.player_quests where player_id=$id and quest_id=$qid" | /usr/bin/grep . || echo none)   prerequisites done: ${q_prereqs:-none}"
        echo "  level: $(sql -e "select exp from $DB.players where id=$id") exp (needs >= $q_minexp)   position: $(sql -e "select concat(world_id,' ',x,' ',y,' ',z) from $DB.players where id=$id")"
    done
    echo; echo "Ready: log in on the server you test, the start npc ${q_npc:-?} is 1.5 m from you."
    echo "Restore: $self restore $name prep-$qid-go   (Java: GS_DB=$JDB $self restore $name prep-$qid-java)"
}

# pktdiff as a native binary (cross-built once in a golang container when there is no local go).
pktdiff_bin() {
    local arch=$(uname -m); [[ $arch == x86_64 ]] && arch=amd64
    local bin=$qd/bin/pktdiff-$arch
    if [[ -x $bin && -z $(find $goapp/cmd/pktdiff -name '*.go' -newer $bin 2>/dev/null | head -1) ]]; then echo $bin; return; fi
    mkdir -p $qd/bin
    if command -v go >/dev/null; then (cd $goapp && go build -o $bin ./cmd/pktdiff) >&2 && { echo $bin; return; }; fi
    local i v
    for v in go-cache-qd go-build-cache-qd; do container volume ls | /usr/bin/grep -q "^$v " || container volume create $v >/dev/null; done
    for i in 1 2 3; do   # a cache volume attaches to one container at a time
        container run --rm --arch arm64 -v go-cache-qd:/go -v go-build-cache-qd:/root/.cache/go-build -v $goapp:/repo:ro -v $qd/bin:/out -w /repo \
            -e CGO_ENABLED=0 -e GOOS=darwin -e GOARCH=$arch -e GOFLAGS=-buildvcs=false docker.io/library/golang:1.25 /usr/local/go/bin/go build -o /out/pktdiff-$arch ./cmd/pktdiff >&2 && { echo $bin; return; }
        sleep 5; done
    return 1
}
# The newest saved log of a backend for a label (refreshed from the sniffer when it runs and holds the marker).
label_log() { # java|go label
    local n=$SNIFF f; [[ $1 == java ]] && n=$SNIFFJ
    if running $n && f=$(zsh $self log $1 $2 2>/dev/null | head -1 | awk '{print $1}') && [[ -f $f ]]; then echo $f; return; fi
    ls -t $qd/logs/$1-*-$2.log 2>/dev/null | head -1
}
do_compare() { # labelJava labelGo [questId] [pktdiff flags]
    local jl=$1 gl=$2 qid=""; shift 2
    if [[ ${1:-} =~ '^[0-9]+$' ]]; then qid=$1; shift; fi
    local jf gf bin
    jf=$(label_log java $jl); gf=$(label_log go $gl)
    [[ -n $jf ]] || die "no Java log for label '$jl'. The Java stack must have run it: $self up; $self mark $jl; play the quest on entry 2 (Java); $self log java $jl"
    [[ -n $gf ]] || die "no Go log for label '$gl'. Run: $self mark $gl; play the same steps on entry 1 (Go); $self log go $gl"
    bin=$(pktdiff_bin) || die "cannot build pktdiff (needs a local go, or the container golang:1.25 image with a free cache volume)"
    echo "java: $jf"; echo "go:   $gf"; echo
    $bin -dialogs -quest "$@" $jf $gf
    local rc=$?
    echo; echo "byte-level detail of one step: $bin -quest -norm id,time,coord -from CM_DIALOG_SELECT $jf $gf   (packets: -h)"
    if [[ -n $qid ]]; then
        echo; echo "== Java handler for quest $qid (scripts/quest-java-table.py; verify rows against the L<line> numbers)"
        AION_JAVA=$src/AL-Game python3 $goapp/scripts/quest-java-table.py $qid
    fi
    return $rc
}

exists() { container ls -a | awk -v n=$1 '$1==n{f=1} END{exit !f}'; }
# One command to a sniffer's control port (:7778); prints its one-line answer.
ctl() { local a; a=$(ip $1); [[ -n $a ]] || return 1; print -r -- "$2" | nc -w 2 $a 7778; }
sniffer_target() { ctl $1 status | sed -n 's/^ok target \([^ ]*\).*/\1/p'; }
# Sniffer $1 relaying to $2 (host:port). A running one is only retargeted, so its IP (and ReRun's relay) stays.
ensure_sniffer() {
    # An old sniffer image has no control port: recreate it (new ip, so ReRun needs Play again).
    # al19-game must carry HOST_NAME=127.0.0.1 or ReRun treats it as foreign and replaces it with the plain Go image.
    if [[ $1 == $SNIFF ]] && ! same_env $1 HOST_NAME=127.0.0.1; then container delete --force $1 >/dev/null 2>&1; fi
    if ! running $1 || [[ -z $(sniffer_target $1) ]]; then
        local e=(); [[ $1 == $SNIFF ]] && e=(-e HOST_NAME=127.0.0.1)
        recreate $1 $e $gamesniff_image -target $2
        local i; for i in {1..20}; do [[ -n $(sniffer_target $1) ]] && break; sleep 1; done
    fi
    [[ $(sniffer_target $1) == $2 ]] || ctl $1 "target $2" >/dev/null
    [[ $(sniffer_target $1) == $2 ]] || die "cannot point $1 at $2 (control port :7778)"
}
same_env() { # container, then NAME=value pairs
    local n=$1 kv; shift; running $n || return 1
    for kv in "$@"; do [[ $(cenv $n ${kv%%=*}) == ${kv#*=} ]] || return 1; done; }
registered() { container logs al19-login 2>&1 | /usr/bin/grep -q "game server registration\" id=$1 ip=$2 response=0"; }

# The Java server has its own schema $JDB (a copy of au_server_gs), so a quest done on one server is still open on the other.
# Creates it when missing; "sync-java" re-copies it from the Go schema (stop-the-world: Java is stopped meanwhile).
copy_java_db() {
    running $JAVA && container stop $JAVA >/dev/null
    sql -e "drop database if exists $JDB; create database $JDB"
    container exec al19-db sh -c "mariadb-dump -uroot -paion --single-transaction --routines au_server_gs | mariadb -uroot -paion $JDB" || die "copying au_server_gs to $JDB failed"
}
ensure_java_db() { [[ -n $(sql -e "show databases like '$JDB'") ]] || copy_java_db; }
ensure_login() {
    local want; want=$(sql -e "select count(*) from au_server_ls.gameservers")
    if ! running al19-login || ! container logs al19-login 2>&1 | /usr/bin/grep 'loaded gameServers' | tail -1 | /usr/bin/grep -qE "gameServers=$want( |$)"; then
        echo "recreating the login server (it must load $want game server rows; its address changes, so game servers follow)"
        recreate al19-login -e AION_DB=$(ip al19-db) $login_go_image; wait_log al19-login "Total Boot Time" 60
    fi
}
ensure_go() {
    local common=(AION_DB=$(ip al19-db) AION_LS=$(ip al19-login) AION_CS=$(ip al19-chat) HOST_NAME=127.0.0.1 AION_GSID=1)
    if ! same_env $GO $common; then local e=() kv; for kv in $common; do e+=(-e $kv); done
        recreate $GO $e -e AION_DEBUG=1 $game_go_image; fi
}
ensure_java() {
    local sj; sj=$(ip $SNIFFJ)
    local common=(AION_DB=$(ip al19-db) AION_LS=$(ip al19-login) AION_CS=$(ip al19-chat) HOST_NAME=$sj AION_GSID=2 AION_GS_DB=$JDB)
    if same_env $JAVA $common && [[ $(javaimg_of) == $(javaimg) ]]; then return; fi
    echo $(javaimg) > $qd/java-image
    local e=(); local kv; for kv in $common; do e+=(-e $kv); done
    if [[ $(javaimg) == source ]]; then recreate $JAVA -m 3G $e $game_java21_image
    else recreate $JAVA --arch amd64 -m 3G $e -e AION_DB_USER=root -e AION_DB_PASSWORD=aion \
            -v $src/docker/old-gs:/patch:ro docker.io/rafabertholdo/aion-gs:1.9 sh /patch/start.sh; fi
}
show_entries() {
    local gi=$(ip $GO) si=$(ip $SNIFF) ji=$(ip $JAVA) sj=$(ip $SNIFFJ)
    echo "Server list the client sees (login order = id order):"
    echo "  entry 1 = id 1 = GO    127.0.0.1:7777  ReRun relay -> $SNIFF ${si:-?} -> $GO ${gi:-?}   registered: $(registered 1 $gi && echo yes || echo NO)"
    echo "  entry 2 = id 2 = JAVA  ${sj:-?}:7777  direct -> $SNIFFJ -> $JAVA ${ji:-?} ($(javaimg) image)   registered: $(registered 2 $ji && echo yes || echo NO)"
}

case ${1:-} in
up)
    container system start >/dev/null
    # db: never removed. Start it only if it exists stopped or is missing entirely.
    if ! running al19-db; then
        if exists al19-db; then container start al19-db >/dev/null
        else container volume ls | /usr/bin/grep -q al19-db-data || container volume create al19-db-data >/dev/null
            container run -d --name al19-db -e MARIADB_ROOT_PASSWORD=aion -v al19-db-data:/var/lib/mysql $db_image >/dev/null; fi
    fi
    until container exec al19-db mariadb-admin -uroot -paion -h127.0.0.1 --protocol=tcp ping >/dev/null 2>&1; do sleep 2; done
    sql -e "insert ignore into au_server_ls.gameservers (id,mask,password) values (1,'*','aion'),(2,'*','aion')"
    ensure_java_db
    ensure_login
    running al19-chat || recreate al19-chat $chat_go_image
    ensure_go
    ensure_sniffer $SNIFF $(ip $GO):7777
    ensure_sniffer $SNIFFJ 127.0.0.1:1     # placeholder: the Java server needs this sniffer's ip first
    ensure_java
    ensure_sniffer $SNIFFJ $(ip $JAVA):7777
    echo "waiting for both game servers to register with the login server (Java takes about a minute)"
    for i in {1..150}; do
        registered 1 $(ip $GO) && registered 2 $(ip $JAVA) && break
        for n in $GO $JAVA; do running $n || { container logs $n 2>&1 | tail -15; die "$n stopped before it registered"; }
            container logs $n 2>&1 | /usr/bin/grep -q 'Critical Error' && { container logs $n 2>&1 | tail -15; die "$n failed to start"; }; done
        sleep 2
    done
    [[ $i == 150 ]] && die "not both registered within 5 minutes (status shows which)"
    date +%Y%m%d-%H%M%S > $qd/up-at
    echo; show_entries; echo
    echo "NOW: press Play ONCE in ReRun (Aion tab, Server: This Mac). Play refreshes its 127.0.0.1 relays to the login,"
    echo "     Go sniffer and chat. Then pick the server in the list: entry 1 = Go, entry 2 = Java."
    echo "Separate game databases: Go uses au_server_gs, Java uses $JDB (a copy made when it was missing; 'sync-java' re-copies"
    echo "it from the Go one, e.g. after creating a character on Go). A quest done on one server is NOT done on the other."
    ;;
sync-java)
    copy_java_db; echo "$JDB re-copied from au_server_gs (Java was stopped: run up to start it)"
    ;;
mark)
    [[ -n ${2:-} && $2 =~ '^[A-Za-z0-9_.:-]+$' ]] || die "usage: mark <label>   (letters, digits, _ . : -)"
    for n in $SNIFF $SNIFFJ; do running $n && echo "$n: $(ctl $n "mark $2")"; done
    ;;
status)
    container ls -a | awk 'NR==1 || $1 ~ /^(al19-|aion-)/'
    echo; show_entries
    for n in $SNIFF $SNIFFJ; do echo "  $n: $(ctl $n status 2>&1)   (log: quest-debug.sh log $([[ $n == $SNIFF ]] && echo go || echo java))"; done
    echo; echo "login sees:"; container logs al19-login 2>&1 | /usr/bin/grep -E 'game server (registration|disconnected)|loaded gameServers' | tail -5
    echo; echo "gameservers:"; sql -e "select id,mask from au_server_ls.gameservers"
    echo "characters (id name account):"; sql -e "select id,name,account_id from $DB.players"
    if [[ -n ${2:-} ]]; then id=$(player $2) || exit 1
        echo "quests of $2 (quest_id status vars complete_count):"; sql -e "select quest_id,status,quest_vars,complete_count from $DB.player_quests where player_id=$id"
        echo "inventory of $2 (itemId count location):"; sql -e "select itemId,itemCount,itemLocation from $DB.inventory where itemOwner=$id and itemLocation=0" | head -40
    fi
    ;;
log)
    [[ ${2:-} == java || ${2:-} == go ]] || die "usage: log java|go [label]"
    n=$SNIFF; [[ $2 == java ]] && n=$SNIFFJ
    exists $n || die "$n does not exist (run up)"
    f=$qd/logs/$2-$(date +%Y%m%d-%H%M%S)${3:+-$3}.log
    container logs $n > $f.all 2>&1
    if [[ -n ${3:-} ]]; then
        line=$(/usr/bin/grep -n "msg=mark label=\"\?$3\"\?\( \|$\)" $f.all | tail -1 | cut -d: -f1)
        [[ -n $line ]] || { rm -f $f.all; die "no marker '$3' in $n's log"; }
        tail -n +$line $f.all > $f; rm -f $f.all
    else mv $f.all $f; fi
    echo "$f  ($(/usr/bin/grep -c 'msg=client\|msg=server' $f) packets)"
    [[ $2 == java ]] && echo "server log: container logs $JAVA" || echo "server log: container logs $GO (AION_DEBUG=1: same packets)"
    ;;
bot)
    [[ ${2:-} == java || ${2:-} == go ]] || die "usage: bot java|go [aionbot flags]"
    b=$2; shift 2
    # go: list id 1 says 127.0.0.1 (ReRun's relay, unreachable from a container), so dial the Go sniffer's ip.
    # java: no override, the bot dials the address the list gives, the Java sniffer's own ip.
    [[ $b == go ]] && flags=(-server 1 -game $(ip $SNIFF):7777) || flags=(-server 2)
    f=$qd/logs/bot-$b-$(date +%Y%m%d-%H%M%S).txt
    container run --rm --arch arm64 -v $qd/bin:/qdbin:ro docker.io/library/golang:1.25 /qdbin/aionbot -login $(ip al19-login):2106 $flags "$@" 2>&1 | tee $f
    echo "bot output: $f"
    ;;
reset-quest)
    [[ -n ${3:-} && $3 =~ '^[0-9]+$' ]] || die "usage: reset-quest <char> <questId> [itemId...]"
    id=$(player $2) || exit 1; n=$3; shift 3
    if [[ ${RESET_TO:-} == START ]]; then
        sql -e "insert into $DB.player_quests (player_id,quest_id,status,quest_vars,complete_count) values ($id,$n,'START',0,0) on duplicate key update status='START',quest_vars=0,complete_count=0; select row_count()"
    else
        sql -e "delete from $DB.player_quests where player_id=$id and quest_id=$n; select row_count()"
    fi
    for item in "$@"; do [[ $item =~ '^[0-9]+$' ]] || die "bad item id $item"
        sql -e "delete from $DB.inventory where itemOwner=$id and itemId=$item; select row_count()"; done
    echo "quest $n reset for player $id (log the character out on both servers first: a live session rewrites its rows)"
    ;;
prep)
    [[ -n ${2:-} && ${3:-} =~ '^[0-9]+$' ]] || die "usage: prep <char> <questId>"
    do_prep $2 $3
    ;;
compare)
    [[ -n ${2:-} && -n ${3:-} ]] || die "usage: compare <labelJava> <labelGo> [questId] [pktdiff flags, e.g. -last-session]"
    shift; do_compare "$@"
    ;;
snapshot)
    do_snapshot $2 ${3:-base}
    ;;
restore)
    id=$(player $2) || exit 1; f=$qd/snapshots/$2-${3:-base}.sql; [[ -f $f ]] || die "no snapshot $f"
    { echo "USE $DB;"; echo "SET FOREIGN_KEY_CHECKS=0;"; echo "START TRANSACTION;"
      echo "delete from $DB.inventory where itemOwner=$id;"
      for t in $(gs_tables); do echo "delete from $DB.$t where player_id=$id;"; done
      sed '1,2d' $f; echo "COMMIT;"; } | sql || die "restore failed (nothing changed: it runs in one transaction)"
    echo "restored $f"
    ;;
down)
    container delete --force $JAVA $SNIFFJ >/dev/null 2>&1
    sql -e "delete from au_server_ls.gameservers where id<>1"
    ensure_login; ensure_go; ensure_sniffer $SNIFF $(ip $GO):7777
    for i in {1..60}; do registered 1 $(ip $GO) && break; sleep 2; done
    echo "Java server, its sniffer and list entry 2 removed; Go-only stack (al19-game -> al19-game-go) runs. db and chat untouched. Press Play in ReRun."
    ;;
*)
    sed -n '2,30p' $0 | sed 's/^# \{0,1\}//'; exit 1
    ;;
esac
