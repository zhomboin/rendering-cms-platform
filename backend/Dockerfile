FROM golang:1.26-alpine AS build

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/rendering-cms-server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S app \
  && adduser -S -G app app \
  && mkdir -p /var/log/rendering-cms-platform \
  && chown -R app:app /var/log/rendering-cms-platform

WORKDIR /app

COPY --from=build /out/rendering-cms-server /app/rendering-cms-server

USER app

EXPOSE 8080

ENTRYPOINT ["/app/rendering-cms-server"]
