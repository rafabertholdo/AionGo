#!/usr/bin/env python3
"""Keep the opted-in local panel connected when ReRun recreates MariaDB.

Run from a user LaunchAgent with StartInterval. Only a panel carrying the
com.aiongo.panel.reconnect=true label is managed; no game containers are changed.
"""
import json
import subprocess
import sys

CONTAINER = "/usr/local/bin/container"
LABEL = "com.aiongo.panel.reconnect"


def reconnect_command(database, panel):
    """Return a replacement command only for a managed panel with a stale address."""
    config = panel["configuration"]
    if config.get("labels", {}).get(LABEL) != "true":
        return None
    if database.get("status") != "running":
        return None
    networks = database.get("networks", [])
    if not networks or not networks[0].get("address"):
        return None
    address = networks[0]["address"].split("/")[0]
    environment = config["initProcess"]["environment"]
    if panel.get("status") == "running" and f"AION_DB={address}" in environment:
        return None
    # This local deployment uses a plain image, without custom mounts or sockets.
    # Do not silently discard them if somebody changes that configuration later.
    if config.get("mounts") or config.get("publishedSockets"):
        raise ValueError("managed panel has custom mounts or sockets; refusing to replace it")
    command = [CONTAINER, "run", "--detach", "--name", "al19-panel"]
    for key, value in config.get("labels", {}).items():
        command += ["--label", f"{key}={value}"]
    for value in environment:
        if not value.startswith("AION_DB="):
            command += ["-e", value]
    command += ["-e", f"AION_DB={address}"]
    for port in config.get("publishedPorts", []):
        command += ["-p", f'{port["hostAddress"]}:{port["hostPort"]}:{port["containerPort"]}/{port["proto"]}']
    command += ["--entrypoint", config["initProcess"]["executable"]]
    command += [config["image"]["reference"]]
    command += config["initProcess"].get("arguments", [])
    return command


def main():
    containers = json.loads(subprocess.check_output(
        [CONTAINER, "list", "--all", "--format", "json"], timeout=15))
    by_name = {c["configuration"]["id"]: c for c in containers}
    if "al19-db" not in by_name or "al19-panel" not in by_name:
        return
    panel = by_name["al19-panel"]
    command = reconnect_command(by_name["al19-db"], panel)
    if command is None:
        return
    if panel.get("status") == "running":
        subprocess.run([CONTAINER, "stop", "al19-panel"], check=True, timeout=30)
    subprocess.run([CONTAINER, "delete", "al19-panel"], check=True, timeout=15)
    subprocess.run(command, check=True, timeout=30)
    print("Reconnected local panel to the current Aion database", flush=True)


if __name__ == "__main__":
    try:
        main()
    except (subprocess.SubprocessError, ValueError, KeyError, OSError) as error:
        print(f"Local panel reconnect: {error}", file=sys.stderr)
        sys.exit(1)
