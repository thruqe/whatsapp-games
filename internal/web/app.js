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
    points: 0,
    wins: 0,
    losses: 0,
    draws: 0,
    pointsDiff: 0
  };

  let selectedGame = (['ttt', 'c4', 'wcg'].includes(initialGame)) ? initialGame : 'ttt';
  let currentRoom = null;
  let isMyTurn = false;
  let mySymbol = '';
  let activeLeaderboardTab = 'global';

  // Elements
  const viewLoading = document.getElementById('viewLoading');
  const viewUsername = document.getElementById('viewUsername');
  const viewLobby = document.getElementById('viewLobby');
  const viewArena = document.getElementById('viewArena');

  const userChip = document.getElementById('userChip');
  const headerUsername = document.getElementById('headerUsername');
  const headerPoints = document.getElementById('headerPoints');
  const headerDiff = document.getElementById('headerDiff');

  const usernameForm = document.getElementById('usernameForm');
  const usernameInput = document.getElementById('usernameInput');
  const usernameError = document.getElementById('usernameError');

  const cardTTT = document.getElementById('cardTTT');
  const cardC4 = document.getElementById('cardC4');
  const cardWCG = document.getElementById('cardWCG');

  const btnFindMatch = document.getElementById('btnFindMatch');
  const queueStatus = document.getElementById('queueStatus');
  const queueTimerText = document.getElementById('queueTimerText');
  const btnCancelQueue = document.getElementById('btnCancelQueue');

  const youSymbol = document.getElementById('youSymbol');
  const youName = document.getElementById('youName');
  const youPoints = document.getElementById('youPoints');
  const oppSymbol = document.getElementById('oppSymbol');
  const oppName = document.getElementById('oppName');
  const oppPoints = document.getElementById('oppPoints');

  const turnBanner = document.getElementById('turnBanner');
  const turnText = document.getElementById('turnText');

  const boardTTT = document.getElementById('boardTTT');
  const boardC4 = document.getElementById('boardC4');
  const c4Grid = document.getElementById('c4Grid');
  const c4ColButtons = document.getElementById('c4ColButtons');

  const boardWCG = document.getElementById('boardWCG');
  const wcgReqLetter = document.getElementById('wcgReqLetter');
  const wcgScore = document.getElementById('wcgScore');
  const wcgStrikes = document.getElementById('wcgStrikes');
  const wcgChainStream = document.getElementById('wcgChainStream');
  const wcgForm = document.getElementById('wcgForm');
  const wcgWordInput = document.getElementById('wcgWordInput');
  const wcgError = document.getElementById('wcgError');

  const modalGameOver = document.getElementById('modalGameOver');
  const resultIcon = document.getElementById('resultIcon');
  const resultTitle = document.getElementById('resultTitle');
  const resultSubtitle = document.getElementById('resultSubtitle');
  const pointsChange = document.getElementById('pointsChange');
  const diffChange = document.getElementById('diffChange');
  const btnPlayAgain = document.getElementById('btnPlayAgain');
  const btnBackToLobby = document.getElementById('btnBackToLobby');

  const btnOpenLeaderboard = document.getElementById('btnOpenLeaderboard');
  const modalLeaderboard = document.getElementById('modalLeaderboard');
  const btnCloseLeaderboard = document.getElementById('btnCloseLeaderboard');
  const leaderboardList = document.getElementById('leaderboardList');
  const lbTabs = document.getElementById('lbTabs');
  const btnResign = document.getElementById('btnResign');

  function showView(view) {
    [viewLoading, viewUsername, viewLobby, viewArena].forEach(v => {
      if (v) v.style.display = (v === view) ? 'flex' : 'none';
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
      const conn = document.getElementById('connectionStatus');
      if (conn) conn.textContent = "Error: Missing bot session ID. Please launch from WhatsApp.";
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
          currentUser.points = msg.payload.points || 0;
          currentUser.wins = msg.payload.wins || 0;
          currentUser.losses = msg.payload.losses || 0;
          currentUser.draws = msg.payload.draws || 0;
          currentUser.pointsDiff = msg.payload.points_diff || 0;
          updateUserDisplay();
          showView(viewLobby);
        } else {
          showView(viewUsername);
        }
        break;

      case 'username_set':
        currentUser.username = msg.payload.username;
        currentUser.points = msg.payload.points || 0;
        currentUser.wins = msg.payload.wins || 0;
        currentUser.losses = msg.payload.losses || 0;
        currentUser.draws = msg.payload.draws || 0;
        currentUser.pointsDiff = msg.payload.points_diff || 0;
        updateUserDisplay();
        showView(viewLobby);
        break;

      case 'matchmaking_status':
        if (msg.payload.status === 'searching') {
          queueStatus.style.display = 'flex';
          btnFindMatch.style.display = 'none';
          startQueueCountdown();
        } else {
          stopQueueCountdown();
          queueStatus.style.display = 'none';
          btnFindMatch.style.display = 'inline-flex';
        }
        break;

      case 'match_start':
        stopQueueCountdown();
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
        } else if (viewArena.style.display === 'flex' && selectedGame === 'wcg' && wcgError) {
          wcgError.textContent = msg.error || 'Invalid word submission';
          setTimeout(() => { if (wcgError) wcgError.textContent = ''; }, 3500);
        } else {
          console.warn("[Game Error]", msg.error);
        }
        break;
    }
  }

  function updateUserDisplay() {
    if (userChip) userChip.style.display = 'flex';
    if (headerUsername) headerUsername.textContent = `@${currentUser.username}`;
    if (headerPoints) headerPoints.textContent = `${currentUser.points} PTS`;
    if (headerDiff) {
      const sign = currentUser.pointsDiff > 0 ? '+' : '';
      headerDiff.textContent = `${sign}${currentUser.pointsDiff}`;
      headerDiff.style.color = currentUser.pointsDiff >= 0 ? 'var(--accent-emerald)' : 'var(--accent-danger)';
    }
  }

  // Username Form Submission
  if (usernameForm) {
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
  }

  // Game Selector
  function selectGameCard(game) {
    selectedGame = game;
    [cardTTT, cardC4, cardWCG].forEach(c => {
      if (!c) return;
      if (c.getAttribute('data-game') === game) {
        c.classList.add('active');
      } else {
        c.classList.remove('active');
      }
    });
  }

  if (cardTTT) cardTTT.addEventListener('click', () => selectGameCard('ttt'));
  if (cardC4) cardC4.addEventListener('click', () => selectGameCard('c4'));
  if (cardWCG) cardWCG.addEventListener('click', () => selectGameCard('wcg'));
  selectGameCard(selectedGame);

  // Matchmaking Buttons & 25s Countdown
  let queueTimerInterval = null;
  let queueSeconds = 0;

  function startQueueCountdown() {
    stopQueueCountdown();
    queueSeconds = 0;
    if (queueTimerText) queueTimerText.textContent = 'Searching for player (0s / 25s)';
    queueTimerInterval = setInterval(() => {
      queueSeconds++;
      if (queueTimerText) {
        if (queueSeconds < 25) {
          queueTimerText.textContent = `Searching for player (${queueSeconds}s / 25s)`;
        } else {
          queueTimerText.textContent = 'Auto-matching with automated bot...';
        }
      }
    }, 1000);
  }

  function stopQueueCountdown() {
    if (queueTimerInterval) {
      clearInterval(queueTimerInterval);
      queueTimerInterval = null;
    }
    queueSeconds = 0;
  }

  if (btnFindMatch) {
    btnFindMatch.addEventListener('click', () => {
      sendAction('find_match', { game_type: selectedGame });
    });
  }

  if (btnCancelQueue) {
    btnCancelQueue.addEventListener('click', () => {
      stopQueueCountdown();
      sendAction('cancel_match', {});
    });
  }

  // Match Lifecycle
  function formatPlayerRating(points, diff) {
    const diffSign = diff > 0 ? '+' : '';
    return `${points} PTS (${diffSign}${diff})`;
  }

  function onMatchStart(payload) {
    currentRoom = payload;
    isMyTurn = payload.your_turn;
    mySymbol = payload.you.symbol;

    const yourUsername = payload.you?.username || currentUser.username || 'You';
    const oppUsername = payload.opponent?.username || (currentRoom?.opponent?.username || 'Opponent');

    if (youSymbol) youSymbol.textContent = payload.you?.symbol || 'X';
    if (youName) youName.textContent = `@${yourUsername}`;
    if (youPoints) youPoints.textContent = formatPlayerRating(payload.you?.points || 0, payload.you?.points_diff || 0);

    if (oppSymbol) oppSymbol.textContent = payload.opponent?.symbol || 'O';
    if (oppName) oppName.textContent = `@${oppUsername}`;
    if (oppPoints) oppPoints.textContent = formatPlayerRating(payload.opponent?.points || 0, payload.opponent?.points_diff || 0);

    updateTurnBanner(isMyTurn);

    // Hide all board containers
    if (boardTTT) boardTTT.style.display = 'none';
    if (boardC4) boardC4.style.display = 'none';
    if (boardWCG) boardWCG.style.display = 'none';

    if (payload.game_type === 'ttt') {
      if (boardTTT) boardTTT.style.display = 'grid';
      renderTTTBoard(payload.board);
    } else if (payload.game_type === 'c4') {
      if (boardC4) boardC4.style.display = 'flex';
      buildC4Grid();
      renderC4Board(payload.board);
    } else if (payload.game_type === 'wcg') {
      if (boardWCG) boardWCG.style.display = 'flex';
      renderWCGBoard(payload.board);
    }

    if (modalGameOver) modalGameOver.style.display = 'none';
    showView(viewArena);
  }

  function updateTurnBanner(myTurn) {
    isMyTurn = myTurn;
    if (turnBanner) {
      if (myTurn) {
        turnBanner.className = 'turn-banner your-turn';
        if (turnText) turnText.textContent = "YOUR TURN";
      } else {
        turnBanner.className = 'turn-banner';
        if (turnText) turnText.textContent = "WAITING FOR OPPONENT...";
      }
    }
  }

  function onMoveUpdate(payload) {
    updateTurnBanner(payload.your_turn);
    if (!currentRoom) return;

    if (currentRoom.game_type === 'ttt') {
      renderTTTBoard(payload.board);
    } else if (currentRoom.game_type === 'c4') {
      renderC4Board(payload.board);
    } else if (currentRoom.game_type === 'wcg') {
      renderWCGBoard(payload.board);
    }
  }

  function onGameOver(payload) {
    if (modalGameOver) modalGameOver.style.display = 'flex';

    const pDelta = payload.points_delta !== undefined ? payload.points_delta : 0;
    const diffDelta = payload.points_diff_delta !== undefined ? payload.points_diff_delta : 0;
    const diffSign = diffDelta > 0 ? '+' : '';

    if (payload.is_winner) {
      if (resultIcon) {
        resultIcon.textContent = "VICTORY";
        resultIcon.style.color = "var(--accent-emerald)";
        resultIcon.style.borderColor = "var(--accent-emerald)";
      }
      if (resultTitle) resultTitle.textContent = "Match Won";
      if (resultSubtitle) resultSubtitle.textContent = "You defeated your opponent.";
      if (pointsChange) {
        pointsChange.textContent = `+${pDelta} PTS`;
        pointsChange.style.color = "var(--accent-emerald)";
      }
      if (diffChange) {
        diffChange.textContent = `${diffSign}${diffDelta}`;
        diffChange.style.color = "var(--accent-emerald)";
      }
    } else if (payload.is_draw) {
      if (resultIcon) {
        resultIcon.textContent = "DRAW";
        resultIcon.style.color = "var(--text-primary)";
        resultIcon.style.borderColor = "var(--surface-border-bright)";
      }
      if (resultTitle) resultTitle.textContent = "Stalemate";
      if (resultSubtitle) resultSubtitle.textContent = "Match concluded in a balanced draw.";
      if (pointsChange) {
        pointsChange.textContent = `+${pDelta} PTS`;
        pointsChange.style.color = "var(--text-primary)";
      }
      if (diffChange) {
        diffChange.textContent = `${diffSign}${diffDelta}`;
        diffChange.style.color = "var(--text-primary)";
      }
    } else {
      if (resultIcon) {
        resultIcon.textContent = "DEFEAT";
        resultIcon.style.color = "var(--accent-danger)";
        resultIcon.style.borderColor = "var(--accent-danger)";
      }
      if (resultTitle) resultTitle.textContent = "Match Lost";
      if (resultSubtitle) resultSubtitle.textContent = `@${payload.winner_name || 'Opponent'} claimed victory.`;
      if (pointsChange) {
        pointsChange.textContent = `+${pDelta} PTS`;
        pointsChange.style.color = "var(--text-secondary)";
      }
      if (diffChange) {
        diffChange.textContent = `${diffSign}${diffDelta}`;
        diffChange.style.color = "var(--accent-danger)";
      }
    }

    currentUser.points += pDelta;
    currentUser.pointsDiff += diffDelta;
    updateUserDisplay();
  }

  // Tic-Tac-Toe Board Interactions
  function renderTTTBoard(board) {
    if (!boardTTT || !board) return;
    const cells = boardTTT.querySelectorAll('.ttt-cell');
    cells.forEach((cell, idx) => {
      const sym = board[idx] || '';
      cell.textContent = sym;
      cell.setAttribute('data-symbol', sym);
    });
  }

  if (boardTTT) {
    boardTTT.addEventListener('click', (e) => {
      if (!isMyTurn) return;
      const cell = e.target.closest('.ttt-cell');
      if (!cell) return;
      const pos = parseInt(cell.getAttribute('data-pos'), 10);
      if (cell.textContent) return; // occupied

      sendAction('make_move', { pos });
    });
  }

  // Connect 4 Board Interactions
  function buildC4Grid() {
    if (!c4Grid) return;
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

  if (c4ColButtons) {
    c4ColButtons.addEventListener('click', (e) => {
      if (!isMyTurn) return;
      const btn = e.target.closest('.c4-drop-btn');
      if (!btn) return;
      const col = parseInt(btn.getAttribute('data-col'), 10);
      sendAction('make_move', { col });
    });
  }

  if (c4Grid) {
    c4Grid.addEventListener('click', (e) => {
      if (!isMyTurn) return;
      const cell = e.target.closest('.c4-cell');
      if (!cell) return;
      const col = parseInt(cell.getAttribute('data-col'), 10);
      sendAction('make_move', { col });
    });
  }

  // Word Chain Game (WCG) Board Interactions
  function renderWCGBoard(board) {
    if (!board) return;

    if (wcgReqLetter) {
      wcgReqLetter.textContent = (board.required_letter || 'A').toUpperCase();
    }

    const isYouP1 = (currentRoom && currentRoom.you && currentRoom.you.symbol === 'X');
    const myScore = isYouP1 ? (board.p1_score || 0) : (board.p2_score || 0);
    const myStrikes = isYouP1 ? (board.p1_strikes || 0) : (board.p2_strikes || 0);

    if (wcgScore) wcgScore.textContent = myScore;
    if (wcgStrikes) wcgStrikes.textContent = `${myStrikes}/3`;

    if (wcgChainStream) {
      const chain = board.chain || [];
      if (chain.length === 0) {
        wcgChainStream.innerHTML = `<div class="wcg-empty-hint">Chain started! Type a word starting with letter <b>${(board.required_letter || 'A').toUpperCase()}</b>.</div>`;
      } else {
        wcgChainStream.innerHTML = chain.map(item => {
          const isYou = (item.symbol === mySymbol);
          const author = item.by || item.player || (isYou ? (currentUser.username || 'You') : (currentRoom?.opponent?.username || 'Opponent'));
          return `
            <div class="wcg-chip ${isYou ? 'you' : 'opp'}">
              <span class="wcg-chip-word">${item.word.toUpperCase()}</span>
              <span class="wcg-chip-author">@${author}</span>
            </div>
          `;
        }).join('');
        wcgChainStream.scrollTop = wcgChainStream.scrollHeight;
      }
    }

    if (wcgWordInput && isMyTurn) {
      wcgWordInput.focus();
    }
  }

  if (wcgForm) {
    wcgForm.addEventListener('submit', (e) => {
      e.preventDefault();
      if (!isMyTurn) return;
      const word = (wcgWordInput.value || '').trim();
      if (!word) return;

      if (wcgError) wcgError.textContent = '';
      sendAction('make_move', { word: word });
      wcgWordInput.value = '';
    });
  }

  // Modal Actions
  if (btnPlayAgain) {
    btnPlayAgain.addEventListener('click', () => {
      if (modalGameOver) modalGameOver.style.display = 'none';
      showView(viewLobby);
      sendAction('find_match', { game_type: selectedGame });
    });
  }

  if (btnBackToLobby) {
    btnBackToLobby.addEventListener('click', () => {
      if (modalGameOver) modalGameOver.style.display = 'none';
      showView(viewLobby);
    });
  }

  if (btnResign) {
    btnResign.addEventListener('click', () => {
      if (confirm("Forfeit this match? Points and diff will be updated accordingly.")) {
        sendAction('leave_game', {});
      }
    });
  }

  // Leaderboard Tabs & Modal
  if (btnOpenLeaderboard) {
    btnOpenLeaderboard.addEventListener('click', () => {
      loadLeaderboard(activeLeaderboardTab);
    });
  }

  if (btnCloseLeaderboard) {
    btnCloseLeaderboard.addEventListener('click', () => {
      if (modalLeaderboard) modalLeaderboard.style.display = 'none';
    });
  }

  if (lbTabs) {
    lbTabs.addEventListener('click', (e) => {
      const tabBtn = e.target.closest('.lb-tab');
      if (!tabBtn) return;
      const game = tabBtn.getAttribute('data-game') || 'global';
      activeLeaderboardTab = game;
      lbTabs.querySelectorAll('.lb-tab').forEach(b => b.classList.remove('active'));
      tabBtn.classList.add('active');
      loadLeaderboard(game);
    });
  }

  async function loadLeaderboard(gameType = 'global') {
    if (modalLeaderboard) modalLeaderboard.style.display = 'flex';
    if (leaderboardList) leaderboardList.innerHTML = '<div class="geist-spinner"></div>';

    try {
      const param = (gameType && gameType !== 'global') ? `?game=${encodeURIComponent(gameType)}` : '';
      const resp = await fetch(`/api/leaderboard${param}`);
      const data = await resp.json();

      if (!leaderboardList) return;

      if (!data || data.length === 0) {
        leaderboardList.innerHTML = '<p class="subtitle" style="text-align: center; padding: 24px 0;">No ranked players yet for this category.</p>';
        return;
      }

      leaderboardList.innerHTML = data.map((entry, idx) => {
        const rankStr = String(idx + 1).padStart(2, '0');
        const isTop = idx === 0;
        const diff = entry.points_diff || 0;
        const diffSign = diff > 0 ? '+' : '';
        const diffClass = diff > 0 ? 'positive' : (diff < 0 ? 'negative' : 'neutral');

        return `
          <div class="lb-item">
            <div class="lb-left">
              <span class="lb-rank" style="${isTop ? 'color: var(--accent-emerald); font-weight: 800;' : ''}">#${rankStr}</span>
              <div>
                <div class="lb-name">@${entry.username || 'Anonymous'}</div>
                <span class="lb-record">${entry.wins}W · ${entry.losses}L · ${entry.draws}D</span>
              </div>
            </div>
            <div class="lb-right">
              <span class="lb-rating">${entry.points} PTS</span>
              <span class="lb-points-diff ${diffClass}">Diff: ${diffSign}${diff}</span>
            </div>
          </div>
        `;
      }).join('');
    } catch (e) {
      if (leaderboardList) {
        leaderboardList.innerHTML = '<p class="subtitle" style="color: var(--accent-danger); text-align: center;">Failed to load leaderboard data.</p>';
      }
    }
  }

  // Initialize
  initSpatialTilt();
  initWebSocket();
})();
