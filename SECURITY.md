# Security

Never put bot tokens, API keys, real Telegram user lists, generated media, private infrastructure details, logs, or database/volume backups into public commits or issue attachments. Use `.env.example` as the credential-free configuration template.

Keep `.env`, `data/`, `runtime/`, and backups accessible only to the deployment operator. The runtime key file is a credential. Docker administrators can inspect container environment variables and mounted data.

If a credential has been committed, revoke or rotate it at its provider first. Removing the current file or adding an ignore rule does not remove it from Git history, forks, or existing clones. Review history and any published artifacts before changing repository visibility.

For a vulnerability, use GitHub private vulnerability reporting if the repository has enabled it. Do not post usable credentials or user data in a public issue. For a non-sensitive bug, provide a minimal reproduction with synthetic data and redacted logs.
