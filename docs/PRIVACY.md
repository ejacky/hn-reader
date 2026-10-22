# HN Reader Privacy Policy

**Last updated:** October 22, 2026

HN Reader is a Chrome extension that adds AI summaries and reading recommendations to Hacker News. It can run in **Cloud mode** (default, no setup) or **Local mode** (you host the server and keep the data yourself).

This policy explains what the cloud service collects and why. If you use Local mode, your reading data stays in your own SQLite database; only the article/comment text you ask to summarize is sent to the LLM provider you configure.

## What we collect in Cloud mode

### Anonymous account

When you install the extension in Cloud mode, the service creates an anonymous account and returns a random token. The token is stored in Chrome extension storage on your device. We do **not** ask for your name, email address, or password. There is no email-based account recovery: if you clear extension storage or uninstall the extension, the old anonymous account cannot be recovered.

### Reading signals

To build recommendations, the extension sends signals such as:

- the Hacker News item id of an article or comment you interact with;
- the action type (for example article clicked, summary viewed, comment viewed, favorite);
- a strength value and, for articles, the title shown on the page;
- timestamps.

### Content sent for summarization

When you request a summary, the extension sends the article URL and title, or the comment text, to the cloud API. The server may fetch the public article page to extract readable text. This content is sent to an LLM provider to generate the summary. Raw fetched page content is not stored long-term; the generated summary is cached.

### Summaries and quota

Generated article summaries, key points, language, and the associated Hacker News item id are stored in a shared summary cache. The cache is keyed by Hacker News item id and language, contains only public article content, and is **not linked to your account**. We also store your quota usage and quota reset date.

### Logs

The server keeps standard request logs (IP address, user agent, timestamp, path, status code, duration) for security, abuse prevention, and debugging. Logs are retained for up to 30 days. API tokens and LLM keys are not written to logs.

## Why we use it

- generate the summaries and recommendations you request;
- enforce the free monthly quota and prevent abuse;
- operate, secure, and debug the service;
- understand aggregate usage so we can improve the product.

We do **not** sell your data, use it for advertising, or use it to train our own models. We do not collect your general browsing history; the extension only runs on `news.ycombinator.com`.

## Sharing

- **LLM provider:** article/comment text and the prompt are sent to the LLM provider configured for the service (currently an OpenAI-compatible provider such as OpenCode Go or DeepSeek) solely to generate the requested summary. The provider's own terms and privacy policy apply.
- **Hosting provider:** the service runs on Tencent Cloud infrastructure.
- **Legal:** we may disclose data if required by applicable law, after a good-faith assessment.

We do not share your anonymous token with third parties.

## Retention and deletion

- Account data (signals, per-user summary records, quota) is kept until you delete your account.
- You can export your data as JSON and delete your cloud account from the extension popup (`我的数据` → `导出我的数据` / `删除云端数据`).
- Deleting the account removes your user row and, through database cascades, your per-user summaries, comments, and signals.
- The shared article summary cache is not tied to a user and may remain after account deletion.
- Backups are retained for up to 14 days and are used only for disaster recovery.
- Server logs are retained for up to 30 days.

## Security

Cloud traffic uses HTTPS. API tokens are random and are stored on your device. No system is perfectly secure; if you believe your token has leaked, delete the account from the popup or contact us.

## Local mode

In Local mode the extension talks to a server you run. Reading signals and summaries are stored in your local SQLite database. If you configure an LLM API key, the article/comment text you summarize is sent to that provider. The developer does not receive this data.

## Children

The service is not directed to children under 13, and we do not knowingly collect personal information from them.

## Changes

We may update this policy. Material changes will be reflected in the "Last updated" date and, where appropriate, in the extension or on the website.

## Contact

Questions or data requests: **support@whoareyou.ren** (replace with the production support address before store submission).

HN Reader is an independent project. Hacker News is a service of Y Combinator; this extension is not affiliated with or endorsed by Y Combinator.
