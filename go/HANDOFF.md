# Development handoff

Read [PORTING.md](PORTING.md) for the authoritative subsystem coverage, quest
coverage, confirmed gaps and verification backlog. Maintain status there;
this handoff does not duplicate counts or assert the running stack's revision.

For quest fixes, read [QUEST_HANDLER_AGENT.md](QUEST_HANDLER_AGENT.md),
[QUEST_AUDIT.md](QUEST_AUDIT.md), [QUEST_TRIAGE.md](QUEST_TRIAGE.md) and
[QUEST_DEBUG.md](QUEST_DEBUG.md). The Java handlers and data are under
`java/AL-Game`; the Go module is `go/`.

Follow the repository's [AGENTS.md](../AGENTS.md) and
[aion-server-expert skill](../.agents/skills/aion-server-expert/SKILL.md).
Run Go commands sequentially. Inspect current source and runtime evidence
before acting on a reported defect or deployment request. Use the image manifest
for versions and tags; preserve existing worktree changes and database volumes.
