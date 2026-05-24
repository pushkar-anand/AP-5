FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /ap5 ./cmd/ap5

# ---

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /ap5 /ap5

ENTRYPOINT ["/ap5"]
CMD ["serve"]
