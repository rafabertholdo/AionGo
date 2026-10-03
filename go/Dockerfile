# The Go servers for Aion 1.9: static binaries on scratch, no OS or runtime.
# Build with scripts/build-images.sh. It stages this module and the separate
# Aion Lightning static data in one container build context.
# Targets: login, chat, game, gamesniff, panel.
FROM golang:1.25 AS build
WORKDIR /src
COPY go/ .
RUN --mount=type=cache,target=/go/pkg/mod CGO_ENABLED=0 GOMAXPROCS=2 go build -p 1 -trimpath -ldflags=-s -o /out/ ./cmd/...

FROM scratch AS login
COPY --from=build /out/loginserver /loginserver
EXPOSE 2106 9014
ENTRYPOINT ["/loginserver"]

FROM scratch AS chat
COPY --from=build /out/chatserver /chatserver
EXPOSE 9021 10241
ENTRYPOINT ["/chatserver"]

FROM scratch AS game
COPY --from=build /out/gameserver /gameserver
COPY java/AL-Game/data/static_data /data/static_data
COPY java/AL-Game/data/packets /data/packets
EXPOSE 7777
ENTRYPOINT ["/gameserver"]

# A decrypting relay for comparing game servers' packets (cmd/gamesniff):
#   container run --name al19-game -e HOST_NAME=127.0.0.1 aion-lightning-gamesniff:1.9 -target <server>:7777
FROM scratch AS gamesniff
COPY --from=build /out/gamesniff /gamesniff
EXPOSE 7777
ENTRYPOINT ["/gamesniff"]

# The website: account sign-up, GM and admin tools (cmd/panel). Point it at the login server's database:
#   container run --detach --name al19-panel -p 127.0.0.1:8080:8080 -e AION_DB=<al19-db ip> aion-lightning-panel:1.9
FROM scratch AS panel
COPY --from=build /out/panel /panel
# The client's item data and UI art (scripts/extract-panel-assets.py); build-images.sh stages it.
COPY panel-assets /assets
EXPOSE 8080
ENTRYPOINT ["/panel"]
