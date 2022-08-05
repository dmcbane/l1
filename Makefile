.PHONY: test clean deps lint all
.PHONY: verbose doc l1-tests

PROG=l1

all: deps test ${PROG} l1-tests lint doc

deps:
	go get .

${PROG}: *.go
	go build .

test:
	go test

l1-tests: ${PROG}
	./l1 tests.l1
	./l1 fact.l1
	./l1 fails.l1 && exit 1 || true

lint:
	golint -set_exit_status .
	staticcheck .

clean:
	rm -f ${PROG}

install: ${PROG}
	go install .

verbose: all
    # The tests are fast!  Just do it again, verbosely:
	go test -v

docker:
	docker build -t ${PROG} .

doc:
	python updatereadme.py
