## Release Title

HN Reader Server v1.0.0 — Windows Installer

---

## Release Notes

### What's in this release

This is the **Windows installer** for HN Reader Server v1.0.0.

The server is a lightweight local service that powers the [HN Reader Chrome Extension](https://chrome.google.com/webstore/detail/hn-reader-ai-reading-assistant/YOUR_EXTENSION_ID). It handles all AI processing — article summaries, comment summaries, and personalized recommendations — using your own LLM API key.

### System Requirements

- Windows 10 or later (64-bit)
- Administrator privileges (required to install the Windows service)

### Installation

1. Download `HNReader-Setup-1.0.0.exe` below.
2. Double-click to run the installer.
3. Enter your LLM API key when prompted (e.g., DeepSeek, OpenAI, or compatible).
4. The installer will automatically register and start the HN Reader service.

The service runs in the background and listens on `http://localhost:8080` by default.

### What gets installed

- `hn-reader.exe` — the server binary (registered as a Windows service)
- `.env` — your configuration (API key, model, port)
- An uninstaller entry in "Add or Remove Programs"

### Upgrading

If you have a previous version installed, simply run the new installer. It will automatically stop and replace the old service while preserving your existing `.env` configuration.

### Uninstallation

Run `uninstall.exe` from the installation directory, or remove "HN Reader Server" from Windows "Add or Remove Programs".

### Source Code

See the full source and build instructions in the [main repository](https://github.com/YOUR_USERNAME/hn-reader).
