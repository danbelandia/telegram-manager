# Build stage
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache de dependencias: solo cambia cuando go.mod/go.sum cambian
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# Runtime stage: imagen minima con el binario + wget para healthcheck
FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=build /out/server /app/server

EXPOSE 8080
CMD ["/app/server"]