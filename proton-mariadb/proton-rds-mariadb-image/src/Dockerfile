FROM mariadb:11.4.7

RUN set -ex; \
    rm -rf /etc/apt/sources.list.d/mariadb.list; \
    ln -s /dev/null $HOME/.mysql_history; \
    apt-get update; \
    apt-get install -y curl jq pv; \
    rm -rf /var/lib/apt/lists/*

ADD my.cnf /etc/mysql/conf.d/my.cnf
RUN chown -R mysql:mysql /etc/mysql/conf.d
VOLUME /var/lib/mysql

COPY pt-query-digest /pt-query-digest
COPY mysqltuner /mysqltuner
COPY entrypoint.sh /entrypoint.sh
COPY report_status.sh /report_status.sh
COPY healthcheck.sh /healthcheck.sh
RUN chmod a+x /entrypoint.sh /report_status.sh /healthcheck.sh /mysqltuner /pt-query-digest

EXPOSE 3306 4444 4567 4568

ENTRYPOINT ["/entrypoint.sh"]

CMD ["mariadbd"]
