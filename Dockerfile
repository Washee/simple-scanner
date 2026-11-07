
# ==========================================
# Stage 1: Go Builder
# ==========================================
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /scanner .


# ==========================================
# Stage 2: Runtime
# ==========================================
FROM alpine:3.24

# Benötigte Systempakete
RUN apk add --no-cache \
    sane-utils \
    sane-backend-hpaio \
    hplip \
    libusb \
    usbutils \
    dbus \
    su-exec \
    && rm -f /etc/sane.d/dll.d/gphoto2

WORKDIR /app

# Benutzer und Gruppen
RUN addgroup -g 10001 app \
    && adduser \
        -D \
        -u 10001 \
        -G app \
        -h /home/app \
        -s /sbin/nologin \
        app \
    && addgroup app lp \
    && mkdir -p /scans /run/dbus \
    && chown app:app /scans \
    && id app \
    && grep -E '^(app|lp):' /etc/group

# Anwendung
COPY --from=builder /scanner ./scanner

# HTML Template
COPY template.html .

# Entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

EXPOSE 8080

CMD ["./scanner"]
