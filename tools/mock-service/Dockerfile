FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mock-service .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/mock-service /app/mock-service
RUN mkdir -p /data
EXPOSE 4300
ENTRYPOINT ["/app/mock-service"]
