# Aion Lightning 1.9's databases on MariaDB 11: au_server_ls (accounts) and
# au_server_gs (characters), created from sql/ on the first start of an empty volume.
FROM mariadb:11
COPY --chmod=644 sql/ /docker-entrypoint-initdb.d/
COPY --chmod=644 aion.cnf /etc/mysql/conf.d/aion.cnf

# Bootstrap an independent Java game schema on a fresh volume. Existing volumes
# are untouched by MariaDB's first-start initialization.
RUN for script in 2-au_server_gs.sql 3-droplist.sql 4-abyss_rank_rev239.sql; do \
      sed 's/au_server_gs/au_server_gs_java/g' "/docker-entrypoint-initdb.d/$script" \
        > "/docker-entrypoint-initdb.d/8-java-$script"; \
    done
