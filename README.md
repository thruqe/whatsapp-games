# WhatsApp Webview Games Server

A real-time multiplayer gaming backend designed specifically for **WhatsApp In-App Webviews**.

Users launch games directly from WhatsApp via the bot command `.games`. The bot issues an authenticated session ID, and users tap the in-app webview link to enter matchmaking, register a unique gamer tag, play live against another player, and climb the global rankings.

---

## 🏛️ Architecture Overview

The system is split into three decoupled services/agents following a strict separation of concerns:

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
|  - Matchmaking Queue (Tic-Tac-Toe, Connect 4)                           |
|  - State machines, turn validation, win/loss/draw detection             |
|  - Broadcasts live board transitions to matched players                 |
|  - Calculates ELO rating adjustments (+25 Win / -15 Loss / +5 Draw)     |
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
+-------------------------------------------------------------------------+
```

---

## 🎯 The Three Core Agents

### 1. Connection Manager Agent (`internal/connmanager`)
- **Session Identification:** Validates and keys every connection strictly by the bot-issued `session_id`.
- **Concurrency & Routing:** Thread-safe registry mapping `sessionID -> *Client` with independent write pumps.
- **Bi-directional Routing:**
  - Forwards incoming JSON actions from clients to the Game Logic Agent.
  - Exposes `SendToSession(sessionID, msg)` and `BroadcastToSessions(sessionIDs, msg)` for outgoing state updates.
- **Heartbeat & Disconnects:** Detects socket drops and notifies Game Logic Agent to resolve active match forfeits.

### 2. Game Logic Agent (`internal/gamelogic`)
- **Username Registration:** Validates that users register a unique username before entering matchmaking.
- **Matchmaking Engine:** Matches pairs of players waiting in queue for a selected game type (`ttt` or `c4`).
- **Game Engines:**
  - **Tic-Tac-Toe (`ttt`):** 3x3 grid, turn alternation (X and O), 3-in-a-row win & draw detection.
  - **Connect 4 (`c4`):** 7x6 grid, gravity column drop mechanics, 4-in-a-row (horizontal, vertical, diagonal) win detection.
- **State Broadcasts:** Sends live board transitions, turn indicators, and timers to both player sessions.
- **Event Dispatching:** Sends completion events with score/rating deltas into the Database Service buffered channel.

### 3. Database Service (`internal/db`)
- **Asynchronous Batch Worker:** Dedicated background goroutine consuming from a buffered channel (`chan InteractionEvent`, capacity: 1024).
- **Batched Persistence:** Groups multiple game results into SQLite transactions every 500ms or 25 events, preventing I/O bottlenecks.
- **Schema:**
  - `users`: `session_id`, `user_jid`, `username` (UNIQUE), `rating` (default 1000), `wins`, `losses`, `draws`.
  - `matches`: `game_type`, `player1_session`, `player2_session`, `player1_name`, `player2_name`, `winner_session`, `moves_count`.
- **Global Leaderboard:** Computes top players by ELO rating and win record.

---

## 🚀 Running the Server

### Standalone Local Server
```bash
go run ./cmd/server
# Server listens on http://localhost:8088
```

### With Cloudflared Public HTTPS Tunnel (For WhatsApp Webview)
WhatsApp mobile clients require a valid public HTTPS endpoint to open in-app webviews. You can start the server with the `--tunnel` flag:
```bash
./bin/server --tunnel
```
The server will start a `cloudflared` quick tunnel and print:
```
🌐 Public Webview HTTPS URL: https://example-subdomain.trycloudflare.com
👉 Set WHATSAPP_GAMES_URL=https://example-subdomain.trycloudflare.com in whatsapp-go
```

---

## 🤖 WhatsApp Bot Integration

In `whatsapp-go`, the bot provides the unified command:
```
.games
.games ttt
.games c4
.games lb
```
The bot creates a cryptographically random session ID for the user and replies with a one-tap in-app webview link:
`https://<server-domain>/?session=gsess_<token>&user=<user_jid>&game=ttt`

---

## 🧪 Testing

Run all unit tests across all agents:
```bash
go test -v ./...
```
