VERSION = 1.0.0

.PHONY: build run dev extension clean install-service uninstall-service start-service release

build:
	cd server && go build -ldflags "-X main.Version=$(VERSION)" -o hn-reader.exe .

run: build
	cd server && .\hn-reader.exe

dev:
	cd server && go run -ldflags "-X main.Version=$(VERSION)" .

extension:
	@echo "Load extension from ./extension in Chrome developer mode"

clean:
	cd server && del hn-reader.exe hn-reader.db
	cd installer && del HNReader-Setup-*.exe

install-service: build
	@echo "Requires Administrator privileges"
	cd server && .\hn-reader.exe -install

uninstall-service: build
	@echo "Requires Administrator privileges"
	cd server && .\hn-reader.exe -uninstall

start-service: build
	cd server && .\hn-reader.exe -run-service

release: build
	cd installer && makensis /DPRODUCT_VERSION=$(VERSION) setup.nsi
	@echo "Installer: installer/HNReader-Setup-$(VERSION).exe"
