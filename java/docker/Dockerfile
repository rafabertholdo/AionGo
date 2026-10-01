# Aion Lightning 1.9 on Java 21. One build, three targets: login, chat, game.
#   container build -f docker/Dockerfile --target game -t aion-lightning-game:1.9 .
# The servers compile their data/scripts at startup with javax.tools, so the
# runtime is a JDK, not a JRE.

FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/root/.m2 mvn -B -q -DskipTests package

FROM eclipse-temurin:21-jdk AS runtime
ENV JAVA_OPTS="-XX:MaxRAMPercentage=75"
RUN useradd --system --home-dir /opt/aion aion
WORKDIR /opt/aion
COPY --chmod=755 docker/entrypoint.sh /usr/local/bin/aion-entrypoint
ENTRYPOINT ["aion-entrypoint"]

FROM runtime AS login
ENV AION_JAR=al-login-1.0.1.jar AION_MAIN=com.aionemu.loginserver.LoginServer
COPY --from=build /src/AL-Login/target/al-login-1.0.1.jar ./
COPY --from=build /src/AL-Login/target/libs libs
COPY AL-Login/config config.template
COPY AL-Login/data data
RUN mkdir -p log && chown -R aion /opt/aion
USER aion
EXPOSE 2106 9014

FROM runtime AS chat
ENV AION_JAR=al-cserver-1.0.1.jar AION_MAIN=com.aionemu.chatserver.ChatServer HOST_NAME=*
COPY --from=build /src/AL-CServer/target/al-cserver-1.0.1.jar ./
COPY --from=build /src/AL-CServer/target/libs libs
COPY AL-CServer/config config.template
RUN mkdir -p log && chown -R aion /opt/aion
USER aion
EXPOSE 9021 10241

FROM runtime AS game
ENV AION_JAR=al-game-1.0.1.jar AION_MAIN=com.aionemu.gameserver.GameServer
COPY --from=build /src/AL-Game/target/al-game-1.0.1.jar ./
COPY --from=build /src/AL-Game/target/libs libs
COPY AL-Game/config config.template
COPY AL-Game/data data
RUN mkdir -p log cache && chown -R aion /opt/aion
USER aion
EXPOSE 7777
