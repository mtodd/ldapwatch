FROM golang:1.9

WORKDIR /go/src/ldapwatch
COPY . .

# we don't need to build the examples
RUN rm -rf ./examples

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["make", "test"]
