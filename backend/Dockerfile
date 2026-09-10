FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ticket-system .

FROM alpine:3.22
RUN adduser -D -u 10001 appuser
USER appuser
COPY --from=build /app/ticket-system /ticket-system
EXPOSE 8080
ENTRYPOINT ["/ticket-system"]
