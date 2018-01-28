default:

run:
	docker build -t ldapwatch-test .
	docker run -t ldapwatch-test

test:
	go test -v
