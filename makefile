BINARY=ai

build:
	go build -o ${BINARY} .

install:
	go install

clean:
	go clean
	rm ${BINARY}

.PHONY: build install clean
