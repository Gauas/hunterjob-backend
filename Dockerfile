FROM golang:1.27-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=readonly -o /api ./cmd/api
FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
