let statusFilter = "";
let tablesCache = {};

const STATUS_LABELS = { waiting: "Esperando", active: "En juego", closed: "Cerrada" };

function myUserId() {
  return localStorage.getItem("user_id") || "";
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function setMessage(text, type) {
  const element = document.getElementById("poker-message");
  element.textContent = text || "";
  element.className = "message" + (type ? " " + type : "");
}

function dollarsToCents(dollars) {
  return Math.round(Number(dollars) * 100);
}

function renderTableRows(tables) {
  const tbody = document.getElementById("tables-rows");
  if (!tables.length) {
    tbody.innerHTML = '<tr><td colspan="5" class="muted">No hay mesas todavía. Crea la primera.</td></tr>';
    return;
  }
  const me = myUserId();
  tbody.innerHTML = tables
    .map((table) => {
      const name = escapeHtml(table.name || "Mesa sin nombre");
      const blinds =
        formatCents(table.small_blind_cents || 0) + " / " + formatCents(table.big_blind_cents || 0);
      const players =
        escapeHtml(String(table.current_players || 0)) + " / " + escapeHtml(String(table.max_players || 0));
      const status = escapeHtml(STATUS_LABELS[table.status] || table.status || "—");
      const canJoin = table.status !== "closed";
      let actions = canJoin
        ? '<button class="btn" data-join="' + escapeHtml(table.table_id || "") + '">Unirse</button>'
        : '<span class="muted">—</span>';
      if (me && table.created_by === me && !table.current_players && table.status !== "closed") {
        actions +=
          ' <button class="btn btn-link" data-close="' + escapeHtml(table.table_id || "") + '">Cerrar</button>';
      }
      return (
        "<tr><td>" + name + "</td>" +
        "<td>" + blinds + "</td>" +
        "<td>" + players + "</td>" +
        "<td>" + status + "</td>" +
        "<td>" + actions + "</td></tr>"
      );
    })
    .join("");
}

async function loadTables() {
  const tbody = document.getElementById("tables-rows");
  try {
    const data = await apiGet("/api/games/poker/tables");
    let tables = data.tables || [];
    tablesCache = {};
    tables.forEach((table) => {
      if (table.table_id) tablesCache[table.table_id] = table;
    });
    if (statusFilter) {
      tables = tables.filter((table) => table.status === statusFilter);
    }
    renderTableRows(tables);
  } catch (error) {
    tbody.innerHTML = '<tr><td colspan="5" class="muted">No se pudieron cargar las mesas.</td></tr>';
    setMessage(error.message, "error");
  }
}

async function createTable(event) {
  event.preventDefault();
  const form = event.target;
  const name = form.name.value.trim();
  const smallBlindCents = dollarsToCents(form.smallBlind.value);
  const bigBlindCents = dollarsToCents(form.bigBlind.value);
  const maxPlayers = Number(form.maxPlayers.value);
  const buyInInput = form.elements.buyIn ? form.elements.buyIn.value : "";
  const buyInCents = buyInInput === "" ? 0 : dollarsToCents(buyInInput);

  if (!name) {
    setMessage("Escribe un nombre para la mesa.", "error");
    return;
  }
  if (bigBlindCents < smallBlindCents * 2) {
    setMessage("La ciega mayor debe ser al menos el doble de la menor.", "error");
    return;
  }
  if (buyInCents < 0) {
    setMessage("El buy-in no puede ser negativo.", "error");
    return;
  }

  const body = {
    name: name,
    small_blind_cents: smallBlindCents,
    big_blind_cents: bigBlindCents,
    max_players: maxPlayers,
  };
  if (buyInCents > 0) {
    body.buy_in_amount_cents = buyInCents;
    body.hold_idempotency_key = crypto.randomUUID();
  }

  try {
    setMessage("");
    await apiPost("/api/games/poker/tables", body);
    form.reset();
    form.maxPlayers.value = 6;
    if (buyInCents > 0) {
      setMessage("Mesa creada y te sentaste automáticamente.", "success");
    } else {
      setMessage("Mesa creada. Únete para jugar.", "success");
    }
    loadTables();
  } catch (error) {
    setMessage(error.message, "error");
  }
}

async function joinTable(tableId) {
  const table = tablesCache[tableId];
  const bigBlind = table ? table.big_blind_cents || 0 : 0;
  const minBuyIn = bigBlind * 10;
  const maxBuyIn = bigBlind * 200;
  const rangeText =
    minBuyIn > 0
      ? " (" + formatCents(minBuyIn) + " – " + formatCents(maxBuyIn) + ")"
      : "";
  const answer = prompt("Buy-in para entrar a la mesa ($)" + rangeText + ":", "100");
  if (answer === null) return;
  const buyInCents = dollarsToCents(answer);
  if (!buyInCents || buyInCents <= 0) {
    setMessage("Buy-in inválido.", "error");
    return;
  }
  try {
    setMessage("");
    await apiPost("/api/games/poker/tables/" + tableId + "/join", {
      buy_in_amount_cents: buyInCents,
      preferred_seat: -1,
      hold_idempotency_key: crypto.randomUUID(),
    });
    window.location.href = "poker-table.html?id=" + encodeURIComponent(tableId);
  } catch (error) {
    setMessage(error.message, "error");
  }
}

async function closeTable(tableId) {
  const table = tablesCache[tableId];
  const tableName = table && table.name ? table.name : "esta mesa";
  if (!window.confirm("¿Cerrar \"" + tableName + "\"? La mesa desaparecerá del lobby.")) {
    return;
  }
  try {
    setMessage("");
    await apiPost("/api/games/poker/tables/" + tableId + "/close");
    setMessage("Mesa cerrada.", "success");
    loadTables();
  } catch (error) {
    setMessage(error.message, "error");
  }
}

document.addEventListener("DOMContentLoaded", () => {
  if (!requireAuth()) return;

  renderNav("poker.html");

  document.getElementById("create-table-form").addEventListener("submit", createTable);
  document.getElementById("status-filter").addEventListener("change", (event) => {
    statusFilter = event.target.value;
    loadTables();
  });
  document.getElementById("refresh-btn").addEventListener("click", loadTables);
  document.getElementById("tables-rows").addEventListener("click", (event) => {
    const joinButton = event.target.closest("[data-join]");
    if (joinButton) {
      joinTable(joinButton.getAttribute("data-join"));
      return;
    }
    const closeButton = event.target.closest("[data-close]");
    if (closeButton) {
      closeTable(closeButton.getAttribute("data-close"));
    }
  });

  loadTables();
});
