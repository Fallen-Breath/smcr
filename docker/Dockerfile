FROM golang:1.20 as builder

WORKDIR /build

ARG GOPROXY
ENV GOPROXY=$GOPROXY

COPY go.mod go.sum ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 go build -o smcr cmd/smcr/smcr.go

FROM alpine:latest
RUN apk add --no-cache tzdata  # so TZ environment works
WORKDIR /app
COPY --from=builder /build/smcr /app/smcr

ENTRYPOINT ["/app/smcr", "-c", "config.yml"]
