# Game commands

The account website's `/help` page lists `/ping` for all players. Signed-in
administrators (account access level 3 or higher) also see all 61 commands from
Java's `data/scripts/system/handlers/admincommands`, including syntax, examples,
category filters and search. The server excludes administrator command metadata
from responses to visitors, players and lower-level GMs.

`go/commands/catalog.go` owns the displayed reference. `scripts/test_commands.py`
compares it with Java's registered names; the game tests check that every listed
command has a handler. `/ping` is CM_PING_REQUEST (0x5a), answered with
SM_PING_RESPONSE (0x7c) and payload 0x04, separately from the keepalive ping.

Commands retain Java's default access requirement of 3. `//configure set admin
COMMAND_<NAME> <level>` can override individual command requirements until the
process restarts. Supported live configuration properties are server name,
player limit, character name pattern, simple second class, HTML welcome and
cross-faction binding; Java-only runtime properties are rejected. Use the panel's
Server options for persistent configuration.

Bookmarks are scoped to the GM character. Titles, drop rules and repeating
announcements use the existing MariaDB schema. Spawn exports are written into
`static_data/spawns/new`; review and install them in the canonical static data
to preserve them across image replacement. `//reload spawn` reads static spawn
data; `//reload_spawn` applies it to live objects while preserving summons,
rifts and kisks. Appearance reset restores the original appearance for the
current session.

Shutdown and restart commands close client connections and wait for character
saves before exiting. Docker's restart policy decides whether the process starts
again; with `restart: unless-stopped`, both commands restart the container.
