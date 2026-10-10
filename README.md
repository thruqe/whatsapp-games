# WhatsApp Webview Games

A real-time multiplayer gaming backend designed specifically for **WhatsApp In-App Webviews**.

Users launch games directly from WhatsApp via the bot command `.games`. The bot issues an authenticated session ID, and users tap the in-app webview link to enter matchmaking, register a unique gamer tag, play live against another player (or intelligent fallback bots), and climb the global rankings.

## Usage

Configuration is managed through environment variables or CLI flags; supported settings include:

| Environment Variable | CLI Flag    | Default    | Description                                             |
| :------------------- | :---------- | :--------- | :------------------------------------------------------ |
| `PORT`               | `--port`    | `8088`     | HTTP and WebSocket port to listen on                    |
| `DB_PATH`            | `--db`      | `games.db` | SQLite database file path                               |
| `TUNNEL`             | `--tunnel`  | `false`    | Automatically launch `cloudflared` quick HTTPS tunnel   |

### Running the Server

Using the [Taskfile](./Taskfile.yml):

```bash
# Compile and run locally
task build
./bin/server

# Or run with cloudflared public HTTPS tunnel enabled
./bin/server --tunnel
```

The server starts a `cloudflared` quick tunnel and prints:
```
🌐 Public Webview HTTPS URL: https://example-subdomain.trycloudflare.com
👉 Set WHATSAPP_GAMES_URL=https://example-subdomain.trycloudflare.com in whatsapp-go
```

### Docker

Build and run using Docker:

```bash
# Build production image
task docker:build

# Run container exposing port 8088
task docker:run
```

Or using standard Docker CLI:

```bash
docker build -t thruqe/whatsapp-games:latest .
docker run -it --rm -p 8088:8088 -v $(pwd)/games.db:/app/bin/games.db thruqe/whatsapp-games:latest
```

## Features

- **Multi-Game Real-Time Engine**:
  - **Tic-Tac-Toe (`ttt`)**: 3x3 grid, turn alternation, winning/draw detection.
  - **Connect 4 (`c4`)**: 7x6 gravity board, horizontal, vertical, and diagonal win detection.
  - **Word Chain Game (`wcg`)**: Fast-paced English vocabulary chain with multi-tier live word reality validation.
- **WCG Multi-API Reality Validation Engine**:
  - Validates word reality against multiple authoritative online dictionary APIs in parallel:
    - **Datamuse API** (`api.datamuse.com`)
    - **Wikimedia / Wiktionary Action Query API** (`en.wiktionary.org/w/api.php`)
    - **Wikimedia / Wiktionary REST Definition API** (`en.wiktionary.org/api/rest_v1`)
    - **Free Dictionary API** (`api.dictionaryapi.dev`)
  - Instant O(1) in-memory cache and embedded core dictionary for sub-millisecond local responses.
  - Automatic fallback resilience in offline or high-latency scenarios.
- **Intelligent Bot Opponents**:
  - Instant fallback matchmaker when waiting queues exceed 25 seconds.
  - Procedural esoteric gamer tag generator (`Abyssal_Valkyrie::Null`, `Celestial_Drifter.ai`, etc.).
  - Strategic bot heuristics for Tic-Tac-Toe, Connect 4, and extensive Word Chain vocabularies.
- **Connection Manager**:
  - Thread-safe WebSocket registry identified exclusively by bot-issued cryptographic session IDs.
  - Heartbeat detection and auto-forfeit handling on client drops.
- **Asynchronous Batch Persistence**:
  - Non-blocking SQLite persistence via buffered interaction channels (`chan InteractionEvent`).
  - Flush batching every 500ms or 25 events to eliminate database I/O bottlenecks.
- **Cloudflared Quick Tunnel Integration**:
  - One-flag public HTTPS provisioning (`--tunnel`) required for native WhatsApp mobile webviews.

## Architecture Overview

The system is decoupled into dedicated agents following a strict separation of concerns:

```
+-------------------------------------------------------------------------+
|                        WhatsApp Mobile Client                           |
|                       (In-App Native Webview)                           |
+------------------------------------+------------------------------------+
                                     | WebSocket (/?session=gsess_...)
                                     v
+-------------------------------------------------------------------------+
|                      Connection Manager Agent                           |
|  - Manages WebSocket lifecycles and connections                         |
|  - Identifies users EXCLUSIVELY via Session ID provided by the bot      |
|  - Routes inbound actions to Game Logic Agent                           |
|  - Routes broadcasts/directed states back to client WebSockets          |
+------------------------------------+------------------------------------+
                                     |
                                     v
+-------------------------------------------------------------------------+
|                         Game Logic Agent                                |
|  - Matchmaking Queues (Tic-Tac-Toe, Connect 4, Word Chain)              |
|  - Multi-API Word Validator (Datamuse, Wiktionary, Free Dictionary)     |
|  - State machines, turn validation, win/loss/draw detection             |
|  - Broadcasts live board transitions to matched players                 |
|  - Automated intelligent bot engines & procedural names                 |
|  - Calculates rating adjustments (+25 Win / -15 Loss / +5 Draw)         |
+------------------------------------+------------------------------------+
                                     |
                                     | Buffered Channel (chan InteractionEvent)
                                     v
+-------------------------------------------------------------------------+
|                        Database Service                                 |
|  - Receives interaction events through non-blocking buffered channel    |
|  - Asynchronous Batch Worker flushes events in SQLite transactions      |
|  - Enforces UNIQUE constraint on player usernames                       |
|  - Tracks persistent ratings, stats, and match histories                |
+------------------------------------+------------------------------------+
```

## WhatsApp Bot Integration

In `whatsapp-go`, the bot provides the unified command:

```
.games
.games ttt
.games c4
.games wcg
.games lb
```

The bot issues a cryptographic session token and returns a one-tap in-app webview link:
`https://<server-domain>/?session=gsess_<token>&user=<user_jid>&game=wcg`

## Contributions

If you want to help make this project better, please take the time to read the [Contributing Guidelines](./docs/CONTRIBUTING.md) and open a pull request with your changes.

If you are using any AI agent for assistance, please refer to the [AGENTS Documentation](./docs/AGENTS.md) guide.

## Acknowledgements

whatsapp-games is inspired and powered by the broader Go, WhatsApp, and gaming ecosystems, with dictionary validation backed by Datamuse, Wikimedia Wiktionary, and the open-source dictionary community.

## Licensing

This project is open source; see the [LICENSE](./LICENSE) file for full details.

## Support this project

If you found this project to be useful in any way, please consider sending a tip:

<a href="https://etherscan.io/address/0x76a5307e573401f2b7189fb721a33dc483e985a8"><img src="https://cdn-icons-png.flaticon.com/128/15301/15301597.png" width="24" height="24" alt="Donate ETH" /></a> `0x76a5307e573401f2b7189fb721a33dc483e985a8`
