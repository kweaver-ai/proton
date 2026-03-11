FROM ubuntu:24.04 AS runtime


FROM runtime AS binary
COPY *.tar.gz /

RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*
RUN curl -sSL https://github.com/etcd-io/etcd/releases/download/v3.3.19/etcd-v3.3.19-linux-$(dpkg --print-architecture).tar.gz | \
    tar zx \
    --no-same-owner -C /usr/local/bin --strip-components=1 \
    etcd-v3.3.19-linux-$(dpkg --print-architecture)/etcdctl \
    etcd-v3.3.19-linux-$(dpkg --print-architecture)/etcd

FROM runtime
COPY --from=binary /usr/local/bin/etcdctl /usr/local/bin/etcdctl
COPY --from=binary /usr/local/bin/etcd /usr/local/bin/etcd

RUN mkdir -p /var/etcd /var/lib/etcd
EXPOSE 2379 2380
RUN apt-get update && apt-get install -y inetutils-ping && rm -rf /var/lib/apt/lists/*
RUN groupadd -r -g 1001 mysql && useradd -r -u 1001 -g mysql mysql
RUN chown -R mysql:mysql /var/lib/etcd
USER mysql
CMD ["/usr/local/bin/etcd"]