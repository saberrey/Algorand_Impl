FROM golang:1.11.0
WORKDIR /go/src/github.com/nyu-distributed-systems-fa18/algorand/server
COPY server .
COPY pb ../pb
COPY google.golang.org /go/src/google.golang.org
COPY golang.org /go/src/golang.org
COPY golang /go/src/github.com/golang

RUN go install -v ./...

EXPOSE 3000 3001
CMD ["server"]
