const tableId = new URLSearchParams(window.location.search).get("id");

const SUIT_SYMBOLS = { H: "♥", D: "♦", C: "♣", S: "♠" };
const RED_SUITS = ["H", "D"];

const STAGE_LABELS = {
  preflop: "Preflop",
  flop: "Flop",
  turn: "Turn",
  river: "River",
  showdown: "Showdown",
  settled: "Mano terminada",
};

let table = null;
let seats = [];
let currentRound = null;
let playersBySeat = {};
let winners = [];
let actionInFlight = false;
let pollTimer = null;
let tickTimer = null;

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function setMessage(text, type) {
  const element = document.getElementById("table-message");
  element.textContent = text || "";
  element.className = "message" + (type ? " " + type : "");
}

function shortUser(userId) {
  if (!userId) return "—";
  const id = String(userId);
  return id.length > 10 ? id.slice(0, 8) + "…" : id;
}

function dollarsToCents(value) {
  return Math.round(Number(value) * 100);
}

function cardHtml(card) {
  if (!card) return "";
  const symbol = SUIT_SYMBOLS[card.suit] || card.suit || "";
  const color = RED_SUITS.includes(card.suit) ? "red" : "black";
  return (
    '<div class="card-tile ' +
    color +
    '"><span class="card-rank">' +
    escapeHtml(card.rank || "") +
    '</span><span class="card-suit">' +
    escapeHtml(symbol) +
    "</span></div>"
  );
}

function myUserId() {
  return localStorage.getItem("user_id") || "";
}

function mySeatPosition() {
  const seat = (seats || []).find((s) => s.user_id === myUserId());
  return seat ? seat.seat_position : null;
}

function myPlayerState() {
  const seatPosition = mySeatPosition();
  if (seatPosition === null) return null;
  return playersBySeat[seatPosition] || null;
}

function isMyTurn() {
  if (!currentRound) return false;
  const stage = currentRound.stage;
  if (stage === "settled" || stage === "showdown") return false;
  const seatPosition = mySeatPosition();
  if (seatPosition === null) return false;
  if (currentRound.current_turn !== seatPosition) return false;
  const player = myPlayerState();
  return !player || player.status === "active";
}

function currentPotCents() {
  if (currentRound && currentRound.stage === "settled" && winners.length) {
    return winners.reduce((sum, winner) => sum + (winner.amount_cents || 0), 0);
  }
  const fromPlayers = Object.keys(playersBySeat).reduce(
    (sum, key) => sum + (playersBySeat[key].total_bet_in_round_cents || 0),
    0
  );
  if (fromPlayers > 0) return fromPlayers;
  return (currentRound && currentRound.pot_size_cents) || 0;
}

function renderSeats() {
  const container = document.getElementById("seats");
  const maxPlayers = (table && table.max_players) || 0;
  const dealerPosition = currentRound ? currentRound.dealer_position : -1;
  const currentTurn = currentRound ? currentRound.current_turn : -1;
  const mySeat = mySeatPosition();

  if (!maxPlayers) {
    container.innerHTML = "";
    return;
  }

  const html = [];
  for (let position = 0; position < maxPlayers; position++) {
    const seat = (seats || []).find((s) => s.seat_position === position);
    const player = playersBySeat[position];
    const occupied = !!seat;
    const isMe = position === mySeat;
    const isTurn = position === currentTurn;
    const isDealer = position === dealerPosition;
    const status = (player && player.status) || (seat && seat.status) || "";
    const folded = status === "folded";
    const allIn = status === "all-in";

    let classes = "seat";
    if (!occupied) classes += " empty";
    if (isMe) classes += " me";
    if (isTurn) classes += " turn";
    if (folded) classes += " folded";
    if (allIn) classes += " all-in";

    let inner = "";
    if (isDealer) inner += '<span class="seat-badge dealer-badge">D</span>';
    if (isMe) inner += '<span class="seat-badge me-badge">Tú</span>';
    if (isTurn) inner += '<span class="seat-badge turn-badge">…</span>';

    inner += '<div class="seat-position muted">Asiento ' + position + "</div>";

    if (occupied) {
      const stack = player && player.stack_cents !== undefined ? player.stack_cents : seat.stack_cents;
      const displayName = isMe
        ? localStorage.getItem("username") || shortUser(seat.user_id)
        : shortUser(seat.user_id);
      inner +=
        '<div class="seat-user">' + escapeHtml(displayName) + "</div>" +
        '<div class="seat-stack">' + formatCents(stack || 0) + "</div>";
      if (player && player.total_bet_in_round_cents) {
        inner += '<div class="seat-bet">Apostado ' + formatCents(player.total_bet_in_round_cents) + "</div>";
      }
      if (folded) inner += '<div class="seat-status">Fold</div>';
      if (allIn) inner += '<div class="seat-status">All-in</div>';
    } else {
      inner += '<div class="seat-user muted">Libre</div>';
    }

    html.push('<div class="' + classes + '">' + inner + "</div>");
  }
  container.innerHTML = html.join("");
}

function renderWinners() {
  const banner = document.getElementById("winners-banner");
  if (!winners.length || !currentRound || currentRound.stage !== "settled") {
    banner.innerHTML = "";
    return;
  }
  banner.innerHTML =
    "<strong>Ganadores:</strong> " +
    winners
      .map(
        (winner) =>
          escapeHtml(shortUser(winner.user_id)) +
          " +" +
          escapeHtml(formatCents(winner.amount_cents || 0)) +
          (winner.pot_type && winner.pot_type !== "main" ? " (" + escapeHtml(winner.pot_type) + ")" : "")
      )
      .join(" · ");
}

function renderTurnLabel() {
  const element = document.getElementById("turn-label");
  if (!currentRound || currentRound.current_turn === undefined || currentRound.current_turn === null) {
    element.textContent = "—";
    return;
  }
  const seat = (seats || []).find((s) => s.seat_position === currentRound.current_turn);
  const who = seat && seat.user_id === myUserId() ? "Tu turno" : "Turno de " + shortUser(seat ? seat.user_id : "");
  let countdown = "";
  if (currentRound.turn_expires_at) {
    const remaining = Math.max(0, Math.floor((new Date(currentRound.turn_expires_at) - Date.now()) / 1000));
    countdown = " (" + remaining + "s)";
  }
  element.textContent = who + countdown;
}

function render() {
  if (!table) return;

  document.getElementById("table-name").textContent = table.name || "Mesa";
  document.getElementById("table-meta").textContent =
    "Ciegas " +
    formatCents(table.small_blind_cents || 0) +
    " / " +
    formatCents(table.big_blind_cents || 0) +
    " · " +
    (table.current_players || 0) +
    "/" +
    (table.max_players || 0) +
    " jugadores";

  document.getElementById("pot-size").textContent = formatCents(currentPotCents());
  document.getElementById("current-bet").textContent = formatCents(
    (currentRound && currentRound.current_bet_cents) || 0
  );
  document.getElementById("stage-label").textContent = currentRound
    ? STAGE_LABELS[currentRound.stage] || currentRound.stage || "—"
    : "Sin mano en curso";

  renderTurnLabel();
  renderWinners();
  renderSeats();

  const community = (currentRound && currentRound.community_cards) || [];
  document.getElementById("community-cards").innerHTML = community.map(cardHtml).join("");

  const holeCards = (currentRound && currentRound.your_hole_cards) || [];
  document.getElementById("hole-cards").innerHTML = holeCards.length
    ? holeCards.map(cardHtml).join("")
    : '<span class="muted">Sin cartas todavía (espera a la siguiente mano).</span>';

  const canAct = isMyTurn();
  document.querySelectorAll("#actions button").forEach((button) => {
    button.disabled = !canAct || actionInFlight;
  });
  document.getElementById("raise-amount").disabled = !canAct || actionInFlight;
}

async function loadState() {
  if (!tableId) return;
  try {
    const data = await apiGet("/api/games/poker/tables/" + encodeURIComponent(tableId));
    table = data.table || table;
    seats = data.seats || [];
    currentRound = data.current_round || null;
    if (!currentRound) {
      playersBySeat = {};
      winners = [];
    }
    setMessage("");
    render();
  } catch (error) {
    setMessage(error.message, "error");
  }
}

function applyUpdate(update) {
  if (!update) return;
  currentRound = {
    round_id: update.round_id,
    stage: update.stage,
    community_cards: update.community_cards || [],
    your_hole_cards: (currentRound && currentRound.your_hole_cards) || [],
    pot_size_cents: update.pot_size_cents || 0,
    current_bet_cents: update.current_bet_cents || 0,
    current_turn: update.current_turn,
    turn_expires_at: update.turn_expires_at,
    dealer_position: update.dealer_position,
    actions: update.actions || [],
  };
  playersBySeat = {};
  (update.players || []).forEach((player) => {
    playersBySeat[player.seat_position] = player;
  });
  winners = update.winners || [];
}

async function doAction(action) {
  if (!currentRound || !currentRound.round_id) {
    setMessage("No hay mano en curso.", "error");
    return;
  }
  let amountCents = 0;
  if (action === "raise") {
    amountCents = dollarsToCents(document.getElementById("raise-amount").value);
    if (!amountCents || amountCents <= (currentRound.current_bet_cents || 0)) {
      setMessage(
        "El raise debe superar la apuesta actual (" + formatCents(currentRound.current_bet_cents || 0) + ").",
        "error"
      );
      return;
    }
  }
  actionInFlight = true;
  render();
  try {
    const data = await apiPost("/api/games/poker/tables/" + encodeURIComponent(tableId) + "/action", {
      round_id: currentRound.round_id,
      action: action,
      amount_cents: amountCents,
    });
    applyUpdate(data.update);
    setMessage("");
    if (action === "raise") document.getElementById("raise-amount").value = "";
  } catch (error) {
    setMessage(error.message, "error");
  } finally {
    actionInFlight = false;
    render();
  }
}

async function leaveTable() {
  try {
    setMessage("");
    await apiPost("/api/games/poker/tables/" + encodeURIComponent(tableId) + "/leave");
    window.location.href = "poker.html";
  } catch (error) {
    setMessage(error.message, "error");
  }
}

document.addEventListener("DOMContentLoaded", () => {
  if (!requireAuth()) return;
  if (!tableId) {
    window.location.href = "poker.html";
    return;
  }

  renderNav("poker-table.html");

  document.getElementById("leave-btn").addEventListener("click", leaveTable);
  document.querySelectorAll("#actions button").forEach((button) => {
    button.addEventListener("click", () => doAction(button.dataset.action));
  });

  loadState();
  pollTimer = setInterval(loadState, 3000);
  tickTimer = setInterval(renderTurnLabel, 1000);

  window.addEventListener("pagehide", () => {
    if (pollTimer) clearInterval(pollTimer);
    if (tickTimer) clearInterval(tickTimer);
  });
});
