FROM golang:1.26-alpine AS build

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/cookies-api ./cmd/cookies-api \
    && CGO_ENABLED=0 go build -trimpath -o /out/cookies-migrate ./cmd/cookies-migrate

FROM alpine:3.23

WORKDIR /app

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/cookies-api /app/cookies-api
COPY --from=build /out/cookies-migrate /app/cookies-migrate
COPY migrations /app/migrations

EXPOSE 18080

CMD ["/bin/sh", "-c", "/app/cookies-migrate && exec /app/cookies-api"]
