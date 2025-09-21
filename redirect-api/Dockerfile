FROM golang:alpine3.22 AS build

WORKDIR /app
COPY go.mod ./
# COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o redirectapi main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root
COPY --from=build /app/redirectapi .
EXPOSE 8080
CMD [ "./redirectapi" ]
