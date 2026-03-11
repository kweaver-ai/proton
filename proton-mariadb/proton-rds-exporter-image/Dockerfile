FROM ubuntu:24.04 AS runtime
FROM golang:1.25 AS builder
WORKDIR /app
COPY . .
RUN go build -o /bin/mysqld-exporter .

FROM runtime
COPY --from=builder /bin/mysqld-exporter /bin/mysqld-exporter
RUN chmod a+x /bin/mysqld-exporter
EXPOSE 9104
RUN groupadd -r -g 1001 mysql && useradd -r -u 1001 -g mysql mysql
USER mysql
ENTRYPOINT  ["/bin/mysqld-exporter"]