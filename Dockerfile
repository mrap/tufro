FROM golang:1.8
ENV SRC_DIR /go/src/github.com/mrap/tufro
COPY . $SRC_DIR
WORKDIR $SRC_DIR/twitter/main
CMD go run main.go
