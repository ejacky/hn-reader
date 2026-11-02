VERSION = 1.3.0

.PHONY: build run dev extension i18n-check clean install-service uninstall-service start-service release

build:
	cd server && go build -ldflags "-X main.Version=$(VERSION)" -o hn-reader.exe ./cmd/hn-reader

run: build
	cd server && .\hn-reader.exe

dev:
	cd server && go run -ldflags "-X main.Version=$(VERSION)" ./cmd/hn-reader

extension:
	@echo "Load extension from ./extension in Chrome developer mode"

i18n-check:
	node scripts/check-i18n.mjs

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
	cd installer && makensis -INPUTCHARSET UTF8 -DPRODUCT_VERSION=$(VERSION) setup.nsi
	@echo "Installer: installer/HNReader-Setup-$(VERSION).exe"
