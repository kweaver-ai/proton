FROM ubuntu:24.04 AS runtime
FROM golang:1.25 AS builder
WORKDIR /app
COPY . .
RUN go build -o /bin/proton-rds-mgmt .

FROM runtime
COPY --from=builder /bin/proton-rds-mgmt /proton-rds-mgmt/proton-rds-mgmt
COPY --from=builder /app/rds-mgmt.yaml /etc/rds-mgmt-conf/rds-mgmt.yaml
WORKDIR /proton-rds-mgmt/
RUN groupadd -r -g 1001 mysql && useradd -r -u 1001 -g mysql mysql
RUN mkdir /var/log/proton-rds-mgmt/ && chown -R mysql:mysql /var/log/proton-rds-mgmt/
RUN mkdir /var/lib/rds-mgmt/ && chown -R mysql:mysql /var/lib/rds-mgmt
USER mysql
CMD [ "./proton-rds-mgmt" ]
