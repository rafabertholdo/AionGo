#!/bin/sh
# Entry for the old aion-gs:1.9 image (Java 6, the real 1.0.1 jars) against al19-db (MariaDB 11).
# Two database-dialect fixes only; no game code or packet changes:
#  1. Connector 5.1 cannot do MariaDB 11's TLS: useSSL=false.
#  2. The DAO scripts (data/scripts/system/database/mysql5, compiled at startup) load only when the
#     server's major version is exactly 5 (MySQL5DAOUtils); MariaDB 11 reports 11, so accept >= 5.
cd /root/gameserver
# Own schema (AION_GS_DB, default au_server_gs) so the Java and Go servers never share character/quest state.
sed -i "s#/au_server_gs[a-z_]*?#/${AION_GS_DB:-au_server_gs}?#" config/network/database.properties
grep -q useSSL config/network/database.properties || sed -i 's/characterEncoding=UTF-8/characterEncoding=UTF-8\&useSSL=false/' config/network/database.properties
sed -i 's/majorVersion == 5/majorVersion >= 5/' data/scripts/system/database/mysql5/MySQL5DAOUtils.java
exec sh StartGS.sh
