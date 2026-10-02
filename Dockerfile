# syntax=docker/dockerfile:1

# The frontend and Go toolchain run on the *build* platform (native) and the Go
# binary is cross-compiled for the target, so multi-arch builds do not emulated
# `npm run build` under QEMU (which is extremely slow).
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/overview ./cmd/overview

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/overview /usr/local/bin/overview
ENV OVERVIEW_ADDR=:5230 \
    OVERVIEW_DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 5230
ENTRYPOINT ["overview"]
