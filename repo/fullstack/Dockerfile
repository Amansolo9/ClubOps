FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN go build -o /app/fullstack_app ./fullstack/cmd

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/fullstack_app /app/fullstack_app
COPY --from=builder /app/fullstack/views /app/fullstack/views
COPY --from=builder /app/fullstack/static /app/fullstack/static

RUN mkdir -p /data /app/fullstack/static/uploads

EXPOSE 8080

CMD ["/app/fullstack_app"]
