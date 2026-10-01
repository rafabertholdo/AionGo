# ReRun Aion rollout

AionGo owns the Go servers, Java 21 reference servers, MariaDB initialization,
static data, and the embedded account/GM/admin website. The renderer is a separate
project at `rafabertholdo/AionD9MT`; renderer archives do not belong in AionGo or
the ReRun source repository.

## Initial image set

The nine Linux arm64 images for server release `0.1.0` are built from commit
`d85c51aca5026c3e56cd10262bfc5d7ca39eeb38`. Exact references and OCI index digests
are recorded in [releases/v0.1.0.json](../releases/v0.1.0.json). Later test,
documentation, and release-tool commits do not change the server binaries.

Tags distinguish client version, service, implementation, and server release:
`1.9-game-go-v0.1.0` and `1.9-game-java21-v0.1.0`, plus a source tag ending in
`sha-d85c51aca502`. The website uses `1.9-panel-go-v0.1.0`. All are under
`docker.io/rafabertholdo/aiongo`; existing Java 6 repositories and their `1.9`
tags are retained. There is no rolling `latest` tag.

Published to [Docker Hub](https://hub.docker.com/r/rafabertholdo/aiongo/tags)
on 2026-10-01. All 18 version/source tags were verified against the built digests;
see [the publication record](../releases/publication-v0.1.0.json).

The publication command used a signed-in Apple container CLI:

```sh
container registry login docker.io
python3 scripts/images.py push --revision d85c51aca5026c3e56cd10262bfc5d7ca39eeb38
```

The publisher checks that every requested remote tag is absent before upload and
refuses overwrites. After a partial publication, resume the remaining roles;
a role whose version tag uploaded but whose source tag did not can be completed
with `container image push` for the exact source reference in the release record.

## Validation and switching ReRun

Use the Go stack by default. Java 21 is a comparison implementation with known
2.0 packet leftovers, while the historical Java 6 image remains the captured
1.9 protocol reference.

The initial image checks include source/revision/version labels, matching
version/source digests, separate Go/Java game schemas on a fresh database,
startup and login/chat registration for both implementations, a Go client bot
creating a character and entering/leaving the world, and HTTP page/artwork
responses from the embedded panel. Unit tests run separately for Go, Java,
release tooling, and ReRun.

ReRun's local Aion role definitions point at the new versioned Go images.
The existing running stack is retained during validation; stop and relaunch it
when switching to the new images. Existing database volumes are reused and
MariaDB first-start SQL does not run again on them. Go uses `au_server_gs` and
Java uses `au_server_gs_java`; prepare the latter through the quest-debug runbook
before Java comparisons on an older volume. Preserve both `al19-db-data` and
`aion-db-data`.

For the next release, increment `image-manifest.json`, commit the runtime source,
build and validate, capture a new digest record, then publish new version/source
tags. Update ReRun's pinned release references in the same rollout.
