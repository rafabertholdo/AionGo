# AionGo development

This standalone repository owns Aion 1.9's Go servers, Java 21 reference servers,
static data, database schema, and Go admin website. Git commits and pushes are
allowed within the user's task scope; the restriction in `the-one` applies only
to that sibling repository.

Go code lives in `go/`; Java source and data live in `java/`. Reuse shared helpers.
Run `go/scripts/run-go.sh go test -json ./...`, `go/scripts/run-go.sh go vet ./...`,
and gofmt for Go changes. Run Maven tests for Java changes. Report exact test
counts and failures. Image tooling tests: `python3 -m unittest discover -s scripts`.

Image names and release versions come from `image-manifest.json`. Never publish
to the old Java 6 image repositories or overwrite any `:1.9` tag. Every release
must have a new version, and its images must retain the source revision label.

Preserve `al19-db-data` and `aion-db-data`, and never remove running game containers
or their mounted volumes as part of a build. Apple's container runtime storage
stays on the internal disk. The Go game database is `au_server_gs`; Java uses
`au_server_gs_java` when both implementations run together.
