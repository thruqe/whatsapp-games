# Security Policy

## 1. Credentials and Secrets

Never commit or hardcode credentials, session tokens, or private authentication keys. This kind of information must not be logged in plaintext either.

- Environment variables or a proper secret manager should be used for configuration.
- Persisted authentication data and session identifiers must be handled securely.

## 2. Memory Safety and Stability

whatsapp-games runs as a long-lived real-time gaming backend, so contributions need to be memory-safe and leak-free. This means:

- No memory leaks or unbounded resource growth.
- No unchecked pointer dereferences.
- Proper cleanup of connections, WebSocket pumps, goroutines, and channels.
- Bounded queues and rate limiting on client actions.

Code that could degrade performance or stability over long uptimes will not be merged.

## 3. No Social Engineering

whatsapp-games exists to enable legitimate multiplayer entertainment on WhatsApp webviews, not deception. Contributions or usage that build phishing flows, pretexting, or any logic designed to trick people into giving up private information are strictly prohibited and will be rejected.

## 4. Acceptable Use

whatsapp-games connects real people playing in-app webview games, so this capability must be treated responsibly.

It must not be used to:

- Harass, threaten, or send abusive content.
- Impersonate players or manipulate leaderboards maliciously.
- Distribute malicious payloads or exploit client webviews.

## Reporting a Vulnerability

If a security issue is found, please contact this [email](mailto:thruqe@outlook.com) rather than disclosing it publicly.
