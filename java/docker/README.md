# AionGo container operations

The standalone repository README and `image-manifest.json` describe the new
Go and Java 21 image releases. From the repository root:

```sh
python3 scripts/images.py build
python3 scripts/images.py push
java/docker/aion-servers.sh up
AION_IMPLEMENTATION=java21 java/docker/aion-servers.sh up
```

The source-tree scripts preserve `al19-db-data`. Do not use the legacy Java 6
images for production. `capture-1.9.sh` and `quest-debug.sh` retain those old
images only as protocol and quest references.
