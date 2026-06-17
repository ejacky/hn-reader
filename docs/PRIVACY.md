# Privacy Policy

**Last updated:** May 27, 2026

## Overview

HN Reader is a Chrome extension that enhances the Hacker News reading experience with AI-powered summaries and personalized content matching. This privacy policy explains how we handle your data.

## What We Don't Do

We do **not** collect, store, or transmit any personal information to our servers. Period.

- We do not track your browsing history.
- We do not collect your Hacker News activity.
- We do not sell or share any data with third parties.
- We do not use cookies or analytics.

## What Data Is Used (and Where It Stays)

### 1. Browser Local Storage

The extension uses Chrome's `storage` API to save the following settings **locally on your device only**:

- Your HN Reader server URL (default: `http://localhost:8080`)
- Your preferred summary language (e.g., Chinese or English)

This data never leaves your browser.

### 2. Communication with Your Local Server

To provide AI summaries and recommendations, the extension sends content from Hacker News pages to **your own locally-hosted HN Reader server** (running on your machine, typically at `localhost:8080`). This includes:

- Article titles and URLs (for generating summaries)
- Comment text (when you click "summarize" on a comment)
- Reading interaction signals (e.g., which articles you viewed or clicked)

**Important:** This communication happens entirely between your browser and your own local server. No data is sent to the extension developer or any third-party service.

## Third-Party Services

The extension itself does not integrate with any third-party services. However, your self-hosted HN Reader server may connect to an LLM API provider (such as DeepSeek or OpenAI) using your own API key. Please refer to your chosen LLM provider's privacy policy for information on how they handle API requests.

## Data Retention

Since we do not collect any data on our servers, there is no data retention policy applicable to us. Any data stored by the extension resides in your browser's local storage and can be cleared at any time by uninstalling the extension or clearing site data for the extension.

## Your Rights

Because all your data stays on your local machine, you have full control over it. You can:

- View or modify your stored settings via the extension's popup interface.
- Uninstall the extension at any time to remove all locally stored settings.
- Stop or uninstall the local HN Reader server at any time.

## Changes to This Policy

If we make any changes to this privacy policy, we will update the "Last updated" date above. Since the extension does not collect any data, significant changes to this policy are unlikely.

## Contact

If you have any questions about this privacy policy, please open an issue on our GitHub repository.
