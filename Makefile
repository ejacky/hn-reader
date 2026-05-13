.PHONY: build run dev extension clean

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
