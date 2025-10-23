APP_NAME := vkcli
SRC := main.go

.PHONY: build install clean run

build:
	go mod tidy
	go build -o $(APP_NAME) $(SRC)

install: build
	sudo mv $(APP_NAME) /usr/local/bin/

run: build
	./$(APP_NAME) projects

clean:
	rm -f $(APP_NAME)

