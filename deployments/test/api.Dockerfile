FROM golang:1.26-alpine AS build

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/cookies-api ./cmd/cookies-api \
    && CGO_ENABLED=0 go build -trimpath -o /out/cookies-migrate ./cmd/cookies-migrate

FROM alpine:3.23

# ffprobe 是米云的硬依赖：开关一打开，服务启动时就检查它在不在，缺了直接退出，
# 不是降级成「不探测」。所以它必须在运行镜像里，装在宿主上不算数。
# ffmpeg 同一个包带出来，视频合成那条链路本来也要用。
#
# 换阿里源：从部署这台机器拉 alpine 官方 CDN 十分钟都装不完，构建会一直卡在
# 这一步；阿里源四十秒。
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirrors.aliyun.com|g' /etc/apk/repositories \
    && apk add --no-cache ffmpeg

WORKDIR /app

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/cookies-api /app/cookies-api
COPY --from=build /out/cookies-migrate /app/cookies-migrate
COPY migrations /app/migrations

EXPOSE 18080

CMD ["/bin/sh", "-c", "/app/cookies-migrate && exec /app/cookies-api"]
