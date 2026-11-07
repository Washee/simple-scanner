FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o scanner .

FROM alpine:3.20

RUN apk add --no-cache \
    sane-utils \
    libsane-hpaio \
    libusb-1.0 \
    usbutils \
    dbus

RUN groupadd --gid 10001 app \
    && useradd \
        --uid 10001 \
        --gid 10001 \
        --groups 7 \
        --create-home \
        --shell /usr/sbin/nologin \
        app \
    && mkdir -p /scans /run/dbus \
    && chown app:app /scans

WORKDIR /app

COPY --from=builder /app/scanner .
COPY template.html .
COPY entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

EXPOSE 8080

CMD ["./scanner"]
