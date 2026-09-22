FROM golang:1.27-alpine AS build
WORKDIR /src
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /api ./cmd/api
FROM alpine:3.22
COPY --from=build /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
