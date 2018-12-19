GH_USER ?= josegonzalez
NAME = ssh-github
HARDWARE = $(shell uname -m)
VERSION ?= 0.5.0

build: clean ssh-github
	mkdir -p build/linux  && GOOS=linux  go build -a -o build/linux/$(NAME)
	mkdir -p build/darwin && GOOS=darwin go build -a -o build/darwin/$(NAME)

clean:
	rm -rf build/* ssh-github

run: ssh-github
	./ssh-github

ssh-github:
	go build

release: build
	rm -rf release && mkdir release
	tar -zcf release/$(NAME)_$(VERSION)_linux_$(HARDWARE).tgz -C build/linux $(NAME)
	tar -zcf release/$(NAME)_$(VERSION)_darwin_$(HARDWARE).tgz -C build/darwin $(NAME)
	gh-release create $(GH_USER)/$(NAME) $(VERSION) $(shell git rev-parse --abbrev-ref HEAD)

.PHONY: build clean deps release run
