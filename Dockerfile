FROM golang:1.14.4 AS builder

WORKDIR /app
COPY go.mod go.sum maillog2db.go ./
RUN go mod download && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o app

FROM scratch AS final

COPY --from=builder /app/app /app

ENTRYPOINT ["/app"]
