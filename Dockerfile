FROM --platform=$BUILDPLATFORM docker.io/golang:1.23 AS server-builder
ARG TARGETPLATFORM
RUN apt-get update -y && \
    apt-get install -y gcc-aarch64-linux-gnu libc6-dev-arm64-cross
WORKDIR /usr/src/app
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux CC=aarch64-linux-gnu-gcc GOARCH=arm64 go build -o dist/main cmd/webapp/main.go

FROM docker.io/debian:stable-slim AS runner
RUN apt-get update -y && apt-get install -y ca-certificates
WORKDIR /app
COPY web web
COPY --from=server-builder /usr/src/app/dist /app
EXPOSE 8080
CMD ["/app/main"]