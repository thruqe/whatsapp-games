// Antigravity x Vercel Webview Games Client
(function() {
  const urlParams = new URLSearchParams(window.location.search);
  const sessionID = urlParams.get('session') || '';
  const userJID = urlParams.get('user') || '';
  const initialGame = urlParams.get('game') || 'ttt';

  let ws = null;
  let currentUser = {
    sessionID: sessionID,
    userJID: userJID,
    username: '',
    rating: 1000
  };

  let selectedGame = initialGame;
  let currentRoom = null;
  let isMyTurn = false;
  let mySymbol = '';

  // Elements
  const viewLoading = document.getElementById('viewLoading');
  const viewUsername = document.getElementById('viewUsername');
  const viewLobby = document.getElementById('viewLobby');
  const viewArena = document.getElementById('viewArena');

  const userChip = document.getElementById('userChip');
  const headerUsername = document.getElementById('headerUsername');
  const headerRating = document.getElementById('headerRating');

  const usernameForm = document.getElementById('usernameForm');
  const usernameInput = document.getElementById('usernameInput');
  const usernameError = document.getElementById('usernameError');

  const cardTTT = document.getElementById('cardTTT');
  const cardC4 = document.getElementById('cardC4');
  const btnFindMatch = document.getElementById('btnFindMatch');
  const queueStatus = document.getElementById('queueStatus');
  const queueStatusText = document.getElementById('queueStatusText');
  const btnCancelQueue = document.getElementById('btnCancelQueue');

  const youSymbol = document.getElementById('youSymbol');
  const youName = document.getElementById('youName');
  const youRating = document.getElementById('youRating');
  const oppSymbol = document.getElementById('oppSymbol');
  const oppName = document.getElementById('oppName');
  const oppRating = document.getElementById('oppRating');

  const turnBanner = document.getElementById('turnBanner');
  const turnText = document.getElementById('turnText');

  const boardTTT = document.getElementById('boardTTT');
  const boardC4 = document.getElementById('boardC4');
  const c4Grid = document.getElementById('c4Grid');

  const modalGameOver = document.getElementById('modalGameOver');
  const resultIcon = document.getElementById('resultIcon');
  const resultTitle = document.getElementById('resultTitle');
  const resultSubtitle = document.getElementById('resultSubtitle');
  const ratingChange = document.getElementById('ratingChange');
  const btnPlayAgain = document.getElementById('btnPlayAgain');
  const btnBackToLobby = document.getElementById('btnBackToLobby');

  const btnOpenLeaderboard = document.getElementById('btnOpenLeaderboard');
  const modalLeaderboard = document.getElementById('modalLeaderboard');
  const btnCloseLeaderboard = document.getElementById('btnCloseLeaderboard');
  const leaderboardList = document.getElementById('leaderboardList');
  const btnResign = document.getElementById('btnResign');

  function showView(view) {
    [viewLoading, viewUsername, viewLobby, viewArena].forEach(v => {
      v.style.display = (v === view) ? 'flex' : 'none';
    });
  }

  // Antigravity 3D Spatial Tilt Interactions
  function initSpatialTilt() {
    const cards = document.querySelectorAll('.game-card, .card');
    cards.forEach(card => {
      card.addEventListener('mousemove', (e) => {
        const rect = card.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        const centerX = rect.width / 2;
        const centerY = rect.height / 2;

        const rotateX = ((centerY - y) / centerY) * 4;
        const rotateY = ((x - centerX) / centerX) * 4;

        card.style.transform = `perspective(1000px) rotateX(${rotateX}deg) rotateY(${rotateY}deg) translateY(-2px)`;
      });

      card.addEventListener('mouseleave', () => {
        card.style.transform = 'perspective(1000px) rotateX(0deg) rotateY(0deg) translateY(0)';
      });
    });
  }

  // Initialize Connection
  function initWebSocket() {
    if (!sessionID) {
      document.getElementById('connectionStatus').textContent = "Error: Missing bot session ID. Please launch from WhatsApp.";
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws?session=${encodeURIComponent(sessionID)}&user=${encodeURIComponent(userJID)}`;

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log("[WS] Connected with session ID:", sessionID);
      sendAction('get_profile', {});
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        handleMessage(msg);
      } catch (e) {
        console.error("[WS] Parse error:", e);
      }
    };

    ws.onclose = () => {
      console.log("[WS] Disconnected, retrying in 2s...");
      setTimeout(initWebSocket, 2000);
    };

    ws.onerror = (err) => {
      console.error("[WS] Error:", err);
    };
  }

  function sendAction(action, payload) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ action, payload }));
    }
  }

  // Handle Server Dispatches
  function handleMessage(msg) {
    switch (msg.type) {
      case 'connected':
        break;

      case 'profile':
        if (msg.payload && msg.payload.username) {
          currentUser.username = msg.payload.username;
          currentUser.rating = msg.payload.rating || 1000;
          updateUserDisplay();
          showView(viewLobby);
        } else {
          showView(viewUsername);
        }
        break;

      case 'username_set':
        currentUser.username = msg.payload.username;
        currentUser.rating = msg.payload.rating || 1000;
        updateUserDisplay();
        showView(viewLobby);
        break;

      case 'matchmaking_status':
        if (msg.payload.status === 'searching') {
          queueStatus.style.display = 'flex';
          btnFindMatch.style.display = 'none';
        } else {
          queueStatus.style.display = 'none';
          btnFindMatch.style.display = 'inline-flex';
        }
        break;

      case 'match_start':
        queueStatus.style.display = 'none';
        btnFindMatch.style.display = 'inline-flex';
        onMatchStart(msg.payload);
        break;

      case 'move_update':
        onMoveUpdate(msg.payload);
        break;

      case 'game_over':
        onGameOver(msg.payload);
        break;

      case 'error':
        if (viewUsername.style.display === 'flex') {
          usernameError.textContent = msg.error || 'Registration failed';
        } else {
          alert("Notice: " + (msg.error || 'Action failed'));
        }
        break;
    }
  }

  function updateUserDisplay() {
    userChip.style.display = 'flex';
    headerUsername.textContent = `@${currentUser.username}`;
    headerRating.textContent = `${currentUser.rating} ELO`;
  }

  // Username Form Submission
  usernameForm.addEventListener('submit', (e) => {
    e.preventDefault();
    const name = usernameInput.value.trim();
    if (name.length < 2) {
      usernameError.textContent = "Username must be at least 2 characters.";
      return;
    }
    usernameError.textContent = "";
    sendAction('set_username', { username: name, user_jid: userJID });
  });

  // Game Selector
  cardTTT.addEventListener('click', () => {
    selectedGame = 'ttt';
    cardTTT.classList.add('active');
    cardC4.classList.remove('active');
  });

  cardC4.addEventListener('click', () => {
    selectedGame = 'c4';
    cardC4.classList.add('active');
    cardTTT.classList.remove('active');
  });

  // Matchmaking Buttons
  btnFindMatch.addEventListener('click', () => {
    sendAction('find_match', { game_type: selectedGame });
  });

  btnCancelQueue.addEventListener('click', () => {
    sendAction('cancel_match', {});
  });

  // Match Lifecycle
  function onMatchStart(payload) {
    currentRoom = payload;
    isMyTurn = payload.your_turn;
    mySymbol = payload.you.symbol;

    youSymbol.textContent = payload.you.symbol;
    youName.textContent = `@${payload.you.username}`;
    youRating.textContent = `${payload.you.rating} ELO`;

    oppSymbol.textContent = payload.opponent.symbol;
    oppName.textContent = `@${payload.opponent.username}`;
    oppRating.textContent = `${payload.opponent.rating} ELO`;

    updateTurnBanner(isMyTurn);

    if (payload.game_type === 'ttt') {
      boardTTT.style.display = 'grid';
      boardC4.style.display = 'none';
      renderTTTBoard(payload.board);
    } else {
      boardC4.style.display = 'flex';
      boardTTT.style.display = 'none';
      buildC4Grid();
      renderC4Board(payload.board);
    }

    modalGameOver.style.display = 'none';
    showView(viewArena);
  }

  function updateTurnBanner(myTurn) {
    isMyTurn = myTurn;
    if (myTurn) {
      turnBanner.className = 'turn-banner your-turn';
      turnText.textContent = "YOUR TURN";
    } else {
      turnBanner.className = 'turn-banner';
      turnText.textContent = "WAITING FOR OPPONENT...";
    }
  }

  function onMoveUpdate(payload) {
    updateTurnBanner(payload.your_turn);
    if (currentRoom.game_type === 'ttt') {
      renderTTTBoard(payload.board);
    } else {
      renderC4Board(payload.board);
    }
  }

  function onGameOver(payload) {
    modalGameOver.style.display = 'flex';

    if (payload.is_winner) {
      resultIcon.textContent = "VICTORY";
      resultIcon.style.color = "var(--accent-emerald)";
      resultIcon.style.borderColor = "var(--accent-emerald)";
      resultTitle.textContent = "Match Won";
      resultSubtitle.textContent = "You defeated your opponent.";
      ratingChange.textContent = `+${payload.rating_delta} ELO`;
      ratingChange.style.color = "var(--accent-emerald)";
    } else if (payload.is_draw) {
      resultIcon.textContent = "DRAW";
      resultIcon.style.color = "var(--text-primary)";
      resultIcon.style.borderColor = "var(--surface-border-bright)";
      resultTitle.textContent = "Stalemate";
      resultSubtitle.textContent = "Match concluded in a balanced draw.";
      ratingChange.textContent = `+${payload.rating_delta} ELO`;
      ratingChange.style.color = "var(--text-primary)";
    } else {
      resultIcon.textContent = "DEFEAT";
      resultIcon.style.color = "var(--accent-danger)";
      resultIcon.style.borderColor = "var(--accent-danger)";
      resultTitle.textContent = "Match Lost";
      resultSubtitle.textContent = `@${payload.winner_name || 'Opponent'} claimed victory.`;
      ratingChange.textContent = `${payload.rating_delta} ELO`;
      ratingChange.style.color = "var(--accent-danger)";
    }

    currentUser.rating = Math.max(100, currentUser.rating + payload.rating_delta);
    updateUserDisplay();
  }

  // Tic-Tac-Toe Board Interactions
  function renderTTTBoard(board) {
    const cells = boardTTT.querySelectorAll('.ttt-cell');
    cells.forEach((cell, idx) => {
      const sym = board[idx] || '';
      cell.textContent = sym;
      cell.setAttribute('data-symbol', sym);
    });
  }

  boardTTT.addEventListener('click', (e) => {
    if (!isMyTurn) return;
    const cell = e.target.closest('.ttt-cell');
    if (!cell) return;
    const pos = parseInt(cell.getAttribute('data-pos'), 10);
    if (cell.textContent) return; // occupied

    sendAction('make_move', { pos });
  });

  // Connect 4 Board Interactions
  function buildC4Grid() {
    c4Grid.innerHTML = '';
    for (let r = 0; r < 6; r++) {
      for (let c = 0; c < 7; c++) {
        const cell = document.createElement('div');
        cell.className = 'c4-cell';
        cell.id = `c4-${r}-${c}`;
        cell.setAttribute('data-col', c);
        c4Grid.appendChild(cell);
      }
    }
  }

  function renderC4Board(board) {
    if (!board) return;
    for (let r = 0; r < 6; r++) {
      for (let c = 0; c < 7; c++) {
        const cell = document.getElementById(`c4-${r}-${c}`);
        if (cell) {
          const sym = board[r][c] || '';
          cell.setAttribute('data-symbol', sym);
        }
      }
    }
  }

  document.getElementById('c4ColButtons').addEventListener('click', (e) => {
    if (!isMyTurn) return;
    const btn = e.target.closest('.c4-drop-btn');
    if (!btn) return;
    const col = parseInt(btn.getAttribute('data-col'), 10);
    sendAction('make_move', { col });
  });

  c4Grid.addEventListener('click', (e) => {
    if (!isMyTurn) return;
    const cell = e.target.closest('.c4-cell');
    if (!cell) return;
    const col = parseInt(cell.getAttribute('data-col'), 10);
    sendAction('make_move', { col });
  });

  // Modal Buttons
  btnPlayAgain.addEventListener('click', () => {
    modalGameOver.style.display = 'none';
    showView(viewLobby);
    sendAction('find_match', { game_type: selectedGame });
  });

  btnBackToLobby.addEventListener('click', () => {
    modalGameOver.style.display = 'none';
    showView(viewLobby);
  });

  btnResign.addEventListener('click', () => {
    if (confirm("Forfeit this match? Rating will be updated accordingly.")) {
      sendAction('leave_game', {});
    }
  });

  // Leaderboard Modal (Vercel Analytics Row Style)
  btnOpenLeaderboard.addEventListener('click', loadLeaderboard);
  btnCloseLeaderboard.addEventListener('click', () => {
    modalLeaderboard.style.display = 'none';
  });

  async function loadLeaderboard() {
    modalLeaderboard.style.display = 'flex';
    leaderboardList.innerHTML = '<div class="geist-spinner"></div>';

    try {
      const resp = await fetch('/api/leaderboard');
      const data = await resp.json();

      if (!data || data.length === 0) {
        leaderboardList.innerHTML = '<p class="subtitle" style="text-align: center;">No ranked players yet. Be the first to compete!</p>';
        return;
      }

      leaderboardList.innerHTML = data.map((entry, idx) => {
        const rankStr = String(idx + 1).padStart(2, '0');
        const isTop = idx === 0;

        return `
          <div class="lb-item">
            <div class="lb-left">
              <span class="lb-rank" style="${isTop ? 'color: var(--accent-emerald); font-weight: 800;' : ''}">#${rankStr}</span>
              <div>
                <div class="lb-name">@${entry.username}</div>
                <span class="lb-record">${entry.wins}W · ${entry.losses}L · ${entry.draws}D</span>
              </div>
            </div>
            <span class="lb-rating">${entry.rating} ELO</span>
          </div>
        `;
      }).join('');
    } catch (e) {
      leaderboardList.innerHTML = '<p class="subtitle" style="color: var(--accent-danger); text-align: center;">Failed to load leaderboard data.</p>';
    }
  }

  // Initialize
  initSpatialTilt();
  initWebSocket();
})();
