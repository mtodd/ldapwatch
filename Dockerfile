FROM golang:1.9

WORKDIR /go/src/ldapwatch
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["make", "test"]
