## Release Title

HN Reader v1.1.0 — Cloud Mode & Dual Backend Support

---

## Release Notes

### What's new in v1.1.0

- **☁️ Zero-Config Cloud Mode**: Install the extension and start using it immediately. HN Reader automatically creates an anonymous cloud account for you.
- **💳 Pay-Only-When-Needed**: Basic features are free within the monthly quota. Upgrade prompts only appear when you reach the limit.
- **🖥️ Local Mode Still Supported**: Privacy-conscious users can switch to local mode from advanced settings; it's kept low-profile so it doesn't burden regular users.
- **🔐 Authentication & Quotas**: Cloud backend supports anonymous sign-up, API key authentication, and monthly usage quotas.
- **🚀 Onboarding**: New users see a streamlined welcome page and in-page tooltip on first visit to Hacker News.
- **🐳 Cloud Server Deployment**: Added `server/cloud/` with PostgreSQL-backed multi-tenant backend and Docker support.

### Extension Installation

1. Install from Chrome Web Store or load `./extension` in developer mode.
2. Choose **Cloud Mode** or **Local Mode** on first run.
3. Visit [news.ycombinator.com](https://news.ycombinator.com) and click `[summary]` next to any article.

### Cloud Server Deployment

```bash
cd server/cloud
cp .env.example .env
# edit .env with DATABASE_URL and LLM_API_KEY
docker-compose up -d
```

---

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
