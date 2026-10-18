let period = "all_time";

const GAMES = ["blackjack", "poker"];
const GAME_LABELS = { blackjack: "Blackjack", poker: "Póker" };

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function setMessage(text, type) {
  const element = document.getElementById("stats-message");
  element.textContent = text || "";
  element.className = "message" + (type ? " " + type : "");
}

function setText(id, text) {
  document.getElementById(id).textContent = text;
}

function setStatTone(id, tone) {
  document.getElementById(id).className = "stat-value " + tone;
}

function formatSignedCents(cents) {
  const value = Number(cents || 0);
  const sign = value > 0 ? "+" : value < 0 ? "-" : "";
  return sign + formatCents(Math.abs(value));
}

async function loadPlayerStats() {
  try {
    const data = await apiGet("/api/stats/player");
    setText("stat-wagered", formatCents(data.total_wagered_cents || 0));
    setText("stat-won", formatCents(data.total_won_cents || 0));

    const net = data.net_profit_cents || 0;
    setText("stat-net", formatSignedCents(net));
    setStatTone("stat-net", net > 0 ? "positive" : net < 0 ? "negative" : "neutral");

    setText("stat-games", String(data.games_played || 0));

    const winRate = Number(data.win_rate) || 0;
    setText("stat-winrate", (winRate * 100).toFixed(1) + "%");

    setText("stat-largest", formatCents(data.largest_win_cents || 0));

    const streak = data.current_streak || 0;
    if (streak > 0) {
      setText("stat-streak", streak + " victoria" + (streak === 1 ? "" : "s"));
      setStatTone("stat-streak", "positive");
    } else if (streak < 0) {
      setText("stat-streak", Math.abs(streak) + " derrota" + (streak === -1 ? "" : "s"));
      setStatTone("stat-streak", "negative");
    } else {
      setText("stat-streak", "—");
      setStatTone("stat-streak", "neutral");
    }

    setText("stat-best", String(data.best_streak || 0));

    setText("stat-updated", data.last_updated ? new Date(data.last_updated).toLocaleString() : "—");
  } catch (error) {
    setMessage(error.message, "error");
  }
}

function renderGameRow(game, data) {
  const label = escapeHtml(GAME_LABELS[game] || game);
  if (!data) {
    return (
      "<tr><td>" + label + '</td><td class="muted">—</td><td class="muted">—</td><td class="muted">—</td><td class="muted">—</td></tr>'
    );
  }
  return (
    "<tr><td>" + label + "</td>" +
    "<td>" + escapeHtml(String(data.games_played || 0)) + "</td>" +
    "<td>" + escapeHtml(formatCents(data.total_wagered_cents || 0)) + "</td>" +
    "<td>" + escapeHtml(formatCents(data.total_won_cents || 0)) + "</td>" +
    "<td>" + escapeHtml(formatCents(data.largest_win_cents || 0)) + "</td></tr>"
  );
}

async function loadGameStats() {
  const tbody = document.getElementById("game-rows");
  const rows = [];
  for (const game of GAMES) {
    let data = null;
    try {
      data = await apiGet("/api/stats/player/" + game);
    } catch (error) {
      data = null;
    }
    rows.push(renderGameRow(game, data));
  }
  tbody.innerHTML = rows.join("");
}

async function loadLeaderboard() {
  const tbody = document.getElementById("leaderboard-rows");
  try {
    const data = await apiGet(
      "/api/stats/leaderboard?period=" + encodeURIComponent(period) + "&limit=10"
    );
    const entries = data.entries || [];
    if (!entries.length) {
      tbody.innerHTML = '<tr><td colspan="4" class="muted">Sin datos para este periodo.</td></tr>';
      return;
    }
    tbody.innerHTML = entries
      .map((entry) => {
        const name =
          entry.username ||
          (entry.user_id ? String(entry.user_id).slice(0, 8) + "…" : "—");
        return (
          "<tr><td>" + escapeHtml(String(entry.rank || "")) + "</td>" +
          "<td>" + escapeHtml(name) + "</td>" +
          "<td>" + escapeHtml(formatCents(entry.score_cents || 0)) + "</td>" +
          "<td>" + escapeHtml(String(entry.games_played || 0)) + "</td></tr>"
        );
      })
      .join("");
  } catch (error) {
    tbody.innerHTML = '<tr><td colspan="4" class="muted">No se pudo cargar el ranking.</td></tr>';
  }
}

document.addEventListener("DOMContentLoaded", () => {
  if (!requireAuth()) return;

  renderNav("stats.html");

  document.getElementById("period-select").addEventListener("change", (event) => {
    period = event.target.value;
    loadLeaderboard();
  });

  loadPlayerStats();
  loadGameStats();
  loadLeaderboard();
});
