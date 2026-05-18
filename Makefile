.PHONY: build run dev extension clean install-service uninstall-service start-service

build:
	cd server && go build -o hn-reader.exe .

run: build
	cd server && .\hn-reader.exe

dev:
	cd server && go run .

extension:
	@echo "Load extension from ./extension in Chrome developer mode"

clean:
	cd server && del hn-reader.exe hn-reader.db

install-service: build
	@echo "Requires Administrator privileges"
	cd server && .\hn-reader.exe -install

uninstall-service: build
	@echo "Requires Administrator privileges"
	cd server && .\hn-reader.exe -uninstall

start-service: build
	cd server && .\hn-reader.exe -run-service
