# step 1: build
FROM golang:1.14.14-alpine3.13 as build-step

# for go mod download
RUN apk add --update --no-cache ca-certificates git

RUN mkdir /go-socket-app
WORKDIR /go-socket-app
COPY go.mod .
#COPY go.sum .

RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o /go/bin/go-socket-app

# -----------------------------------------------------------------------------
# step 2: exec
FROM scratch
COPY --from=build-step /go/bin/go-socket-app /go/bin/go-socket-app
ENTRYPOINT ["/go/bin/go-socket-app"]
