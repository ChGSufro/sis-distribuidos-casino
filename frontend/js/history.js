let offset = 0;
let limit = 20;
let gameType = "";
let totalCount = 0;

const ACTION_LABELS = {
  bet_placed: "Apuesta",
  hand_result: "Resultado",
  hand_cancelled: "Mano cancelada",
  table_result: "Resultado de mesa",
  player_buyin: "Entrada a mesa",
  player_payout: "Pago",
  player_eliminated: "Eliminado",
};

const GAME_LABELS = {
  blackjack: "Blackjack",
  poker: "Póker",
};

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function setMessage(text, type) {
  const element = document.getElementById("history-message");
  element.textContent = text || "";
  element.className = "message" + (type ? " " + type : "");
}

function formatTimestamp(value) {
  if (!value) return "";
  const date = new Date(value);
  if (isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function labelFor(map, value) {
  return map[value] || value || "";
}

const SUIT_SYMBOLS = { H: "♥", D: "♦", C: "♣", S: "♠" };

function cardText(card) {
  if (!card) return "";
  const symbol = SUIT_SYMBOLS[card.suit] || card.suit || "";
  return (card.rank || "") + symbol;
}

function cardsText(cards) {
  if (!Array.isArray(cards) || !cards.length) return "";
  return cards.map(cardText).join(" ");
}

function parseDetails(raw) {
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch (error) {
    return null;
  }
}

function formatDetailValue(key, value) {
  if (value === null || value === undefined) return String(value);
  if (typeof value === "object") return JSON.stringify(value);
  if (key.indexOf("_cents") !== -1 && typeof value === "number") {
    return formatCents(value);
  }
  return String(value);
}

function rawDetailsText(raw) {
  const parsed = parseDetails(raw);
  if (parsed === null) return raw || "";
  if (typeof parsed !== "object" || parsed === null) return String(parsed);
  return Object.keys(parsed)
    .map((key) => key + ": " + formatDetailValue(key, parsed[key]))
    .join(", ");
}

function renderHandResult(parsed) {
  const subHands = Array.isArray(parsed.subHands) ? parsed.subHands : [];
  const totalBet = subHands.reduce(
    (sum, subHand) => sum + (Number(subHand.bet_amount_cents) || 0),
    0
  );
  const totalWin = Number(parsed.totalWin) || 0;

  let html = '<div class="detail-summary">';
  if (subHands.length) {
    html += "<span>Apuesta: " + escapeHtml(formatCents(totalBet)) + "</span>";
    html += "<span>Ganado: " + escapeHtml(formatCents(totalWin)) + "</span>";
  }
  html += "</div>";

  if (subHands.length) {
    html += '<div class="detail-hands">';
    subHands.forEach((subHand) => {
      const result = subHand.result || subHand.status || "";
      const cls = "result " + result;
      html +=
        '<span class="' +
        escapeHtml(cls) +
        '">Mano ' +
        escapeHtml(subHand.index) +
        ": " +
        escapeHtml(result) +
        "</span>";
    });
    html += "</div>";
  }

  const dealerCards = parsed.dealerCards;
  const hasCards =
    cardsText(dealerCards) || subHands.some((subHand) => cardsText(subHand.cards));
  if (hasCards) {
    html += '<details class="detail-cards"><summary>Ver cartas</summary>';
    if (cardsText(dealerCards)) {
      html += "<div>Dealer: " + escapeHtml(cardsText(dealerCards)) + "</div>";
    }
    subHands.forEach((subHand) => {
      const text = cardsText(subHand.cards);
      if (text) {
        html +=
          "<div>Mano " + escapeHtml(subHand.index) + ": " + escapeHtml(text) + "</div>";
      }
    });
    html += "</details>";
  }

  return html;
}

function renderDetails(event) {
  const raw = event.details;
  if (!raw) return "";
  const parsed = parseDetails(raw);
  if (
    parsed &&
    typeof parsed === "object" &&
    event.action === "hand_result" &&
    Array.isArray(parsed.subHands)
  ) {
    return renderHandResult(parsed);
  }
  return escapeHtml(rawDetailsText(raw));
}

function renderRows(events) {
  const tbody = document.getElementById("history-rows");
  if (!events || !events.length) {
    tbody.innerHTML =
      '<tr><td colspan="4" class="muted">No hay eventos para mostrar.</td></tr>';
    return;
  }
  tbody.innerHTML = events
    .map(
      (event) =>
        "<tr>" +
        "<td>" + escapeHtml(formatTimestamp(event.timestamp)) + "</td>" +
        "<td>" + escapeHtml(labelFor(GAME_LABELS, event.game_type)) + "</td>" +
        "<td>" + escapeHtml(labelFor(ACTION_LABELS, event.action)) + "</td>" +
        '<td class="details">' + renderDetails(event) + "</td>" +
        "</tr>"
    )
    .join("");
}

function renderPagination() {
  const prevButton = document.getElementById("prev-btn");
  const nextButton = document.getElementById("next-btn");
  const info = document.getElementById("page-info");

  prevButton.disabled = offset <= 0;
  nextButton.disabled = offset + limit >= totalCount;

  if (totalCount === 0) {
    info.textContent = "0 resultados";
    return;
  }
  const from = offset + 1;
  const to = Math.min(offset + limit, totalCount);
  info.textContent = "Mostrando " + from + "–" + to + " de " + totalCount;
}

async function loadHistory() {
  setMessage("");
  try {
    let path = "/api/history?limit=" + limit + "&offset=" + offset;
    if (gameType) path += "&game_type=" + encodeURIComponent(gameType);
    const data = await apiGet(path);
    totalCount = data.total_count || 0;
    renderRows(data.events || []);
    renderPagination();
  } catch (error) {
    setMessage(error.message, "error");
  }
}

function changePage(delta) {
  const nextOffset = offset + delta * limit;
  if (nextOffset < 0 || nextOffset >= totalCount) return;
  offset = nextOffset;
  loadHistory();
}

document.addEventListener("DOMContentLoaded", () => {
  if (!requireAuth()) return;

  renderNav("history.html");

  document.getElementById("game-filter").addEventListener("change", (event) => {
    gameType = event.target.value;
    offset = 0;
    loadHistory();
  });

  document.getElementById("limit-select").addEventListener("change", (event) => {
    limit = Number(event.target.value);
    offset = 0;
    loadHistory();
  });

  document.getElementById("prev-btn").addEventListener("click", () => changePage(-1));
  document.getElementById("next-btn").addEventListener("click", () => changePage(1));

  loadHistory();
});
