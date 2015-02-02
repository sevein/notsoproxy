default: build run

build:
	go get -v ./...
	go install ./...

run:
	notsoproxy
