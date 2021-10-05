FROM golang:1.15 AS builder

WORKDIR /build

COPY . .

RUN go build ./cmd/main.go

FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install postgresql-12 -y

USER postgres

COPY ./internal/database/init.sql .

RUN service postgresql start && \
    psql -c "CREATE USER proxyuser WITH superuser login password 'proxyuser';" && \
    psql -c "ALTER ROLE proxyuser WITH PASSWORD 'proxyuser';" && \
    createdb -O proxyuser proxydb && \
    psql -d proxydb < ./init.sql && \
    service postgresql stop

VOLUME ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]

USER root

WORKDIR /proxy
COPY --from=builder /build/main .

COPY . .

EXPOSE 8080
EXPOSE 8000
EXPOSE 5432

ENV PROXY_PORT=8080
ENV REPEATER_PORT=8000
ENV DB_USER=proxyuser
ENV DB_NAME=proxydb

COPY /genCerts/ca.crt /usr/local/share/ca-certificates
COPY /genCerts/ca.crt /etc/ssl/certs/
RUN apt-get update
RUN apt-get install -y ca-certificates
RUN update-ca-certificates

CMD service postgresql start && ./main
