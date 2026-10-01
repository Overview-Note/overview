# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/overview ./cmd/overview

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/overview /usr/local/bin/overview
ENV OVERVIEW_ADDR=:5230 \
    OVERVIEW_DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 5230
ENTRYPOINT ["overview"]
