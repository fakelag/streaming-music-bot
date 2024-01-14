FROM golang:1.21-alpine as BUILDER

ARG DISCORD_TOKEN

WORKDIR /app
COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . ./
RUN GOOS=linux go build -o main main.go

FROM alpine:3.16
WORKDIR /app

COPY --from=BUILDER /app /app
RUN apk --update add --no-cache python3 py3-pip
RUN python3 -m pip install -U yt-dlp
ENTRYPOINT ["/app/main"]
