claude:
	@claude --dangerously-skip-permissions

# Local dev stack for in-client testing. The stack script and images are local
# to this machine (.build/ is untracked); the game binary is rebuilt only when
# Go sources change.
dev_bin := .build/dev-bin
go_sources := $(shell find go -name '*.go' -not -path '*/.build/*') go/go.mod go/go.sum

.PHONY: run container-up
run: container-up $(dev_bin)/gameserver
	@$(dev_bin)/start-dev.sh

container-up:
	@container system status >/dev/null 2>&1 || container system start

$(dev_bin)/gameserver: $(go_sources) | container-up
	go/scripts/run-go.sh go build -o /repo/.build/gameserver-new ./cmd/gameserver
	mv go/.build/gameserver-new $@
