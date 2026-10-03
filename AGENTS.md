# AionGo development

This standalone repository owns Aion 1.9's Go servers, Java 21 reference servers,
static data, database schema, and Go admin website. Git commits and pushes are
allowed within the user's task scope; the restriction in `the-one` applies only
to that sibling repository.

This repository is public on GitHub. Never write private infrastructure or access
details into repository files, including documentation, scripts, tests, fixtures,
logs, or generated artifacts. This includes credentials, tokens, private keys,
AWS account/profile/instance identifiers, SSH connection details, and private
deployment addresses, paths, or backup locations. Use generic placeholders in
public examples and keep operational notes and secrets outside the repository.
Do not copy private configuration from sibling repositories or local machine
settings into this repository. Before committing or pushing, review the full
diff and new files for private information; do not include secrets in commit
messages, PR descriptions, or published tool output. If existing repository
content appears to expose private information, flag it to the user without
repeating the sensitive values; do not rewrite Git history without authorization.

For Go coding, review, debugging and setup, read `.agents/skills/golang-how-to/SKILL.md`
and the relevant topic skills. Apply them within the module's Go 1.25 target,
preserving Aion 1.9 packet bytes, opcodes, ordering, rounding and login crypto.
Reuse explicit SQL, manual wiring, existing helpers and standard-library tests.
Preserve meaningful nil/empty distinctions. Avoid blanket style rewrites or new
dependencies without a demonstrated need. Run Go commands sequentially on this
host and use disposable fixtures for database tests. Follow
`docs/GO_SKILLS_APPLICATION_PLAN.md` for the critical pass and review coverage.

Go code lives in `go/`; Java source and data live in `java/`. Reuse shared helpers.
Run `go/scripts/run-go.sh go test -json ./...`, `go/scripts/run-go.sh go vet ./...`,
and gofmt for Go changes. Run Maven tests for Java changes. Report exact test
counts and failures. Image tooling tests: `python3 -m unittest discover -s scripts`.

Deployment requests are execution requests: when the user says to deploy or
publish a build, carry out the release and deployment steps. Tests belong in the
development phase; do not run tests as part of deployment unless the user
explicitly asks for deployment-time testing.

For quest ports, use bounded integration batches: run each handler's focused
tests as it is implemented, then format the batch and run the repository-wide
test suite and vet once after all handlers and shared registrations are
integrated. Build one local game image for the completed batch. If code changes
after the full checks, rerun the suite and vet before handoff. Run Go commands
sequentially because Apple's container runtime can attach the shared Go cache
volumes to only one container at a time. See `go/QUEST_HANDLER_AGENT.md` for
the full batch cadence. If the image script's clean-tree guard rejects shared
quest edits, keep the worktree intact and build a local-only game image with
`container build --no-cache --platform linux/arm64 --progress plain --cpus 2
--memory 4G -f go/Dockerfile --target game -t <manifest-derived-local-tag> .`.
Use a worktree-content hash in its local tag/revision label; never publish it
or restart the server stack for quest verification.

Image names and release versions come from `image-manifest.json`. Never publish
to the old Java 6 image repositories or overwrite any `:1.9` tag. Every release
must have a new version, and its images must retain the source revision label.

Preserve `al19-db-data` and `aion-db-data`, and never remove running game containers
or their mounted volumes as part of a build. Apple's container runtime storage
stays on the internal disk. The Go game database is `au_server_gs`; Java uses
`au_server_gs_java` when both implementations run together.
