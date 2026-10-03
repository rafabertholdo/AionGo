#!/bin/zsh
# Java-vs-Go quest parity: dump what every Java quest handler does for a sweep of states, npcs and dialogs
# (QuestTraceDump, no database or client), then replay it against the Go handlers (TestQuestJavaParity).
#   scripts/quest-parity.sh                 every quest
#   scripts/quest-parity.sh 1001 1002       only these
#   SKIP_JAVA=1 scripts/quest-parity.sh ... reuse the last Java dump (Go-only change)
set -eu
setopt pipefail
module_dir=$(cd "${0:A:h}/.." && pwd)
repo_dir=${module_dir:h}
mkdir -p "$module_dir/.build"
ids=${(j:,:)@}
if [[ -z ${SKIP_JAVA:-} ]]; then
    container run --rm --arch arm64 --memory 4G -v m2-cache:/root/.m2 -v "$repo_dir/java:/src" \
        -v "$module_dir/.build:/out" -w /src maven:3.9-eclipse-temurin-21 \
        mvn -B -q -o test -pl AL-Game -am -Dsurefire.failIfNoSpecifiedTests=false -Dtest=QuestTraceDump \
        -DquestTrace=/out/java_quest_traces.jsonl -DquestIds="$ids" 2>&1 | grep -v '^> ' \
        || { print -u2 'Java dump failed'; exit 1; }
fi
run=TestQuestJavaParity
(( $# )) && run="TestQuestJavaParity/^(${(j:|:)@})\$"
exec "$module_dir/scripts/run-go.sh" go test ./game -run "$run" -count=1 ${PARITY_V:+-v}
