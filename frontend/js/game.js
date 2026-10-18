let handId = null;
let currentHand = null;
let selectedSubHand = 0;

const SUIT_SYMBOLS = { H: "♥", D: "♦", C: "♣", S: "♠" };
const RED_SUITS = ["H", "D"];

function setMessage(text, type) {
  const element = document.getElementById("game-message");
  element.textContent = text || "";
  element.className = "message" + (type ? " " + type : "");
}

function dollarsToCents(value) {
  return Math.round(Number(value) * 100);
}

function isSettled(status) {
  return status === "settled" || status === "cancelled";
}

function firstActiveIndex(subHands) {
  const index = (subHands || []).findIndex((subHand) => subHand.status === "active");
  return index >= 0 ? index : 0;
}

function cardHtml(card) {
  if (!card || card.hidden) {
    return '<div class="card-tile card-back"></div>';
  }
  const symbol = SUIT_SYMBOLS[card.suit] || card.suit;
  const color = RED_SUITS.includes(card.suit) ? "red" : "black";
  return (
    '<div class="card-tile ' +
    color +
    '"><span class="card-rank">' +
    card.rank +
    '</span><span class="card-suit">' +
    symbol +
    "</span></div>"
  );
}

function handTotal(cards) {
  let total = 0;
  let aces = 0;
  (cards || []).forEach((card) => {
    if (card.hidden) return;
    const value = card.value || 0;
    total += value;
    if (value === 11) aces += 1;
  });
  while (total > 21 && aces > 0) {
    total -= 10;
    aces -= 1;
  }
  return total;
}

function setActionsEnabled(enabled) {
  document.querySelectorAll(".actions button").forEach((button) => {
    button.disabled = !enabled;
  });
}

function setAction(action, enabled) {
  const button = document.querySelector('.actions button[data-action="' + action + '"]');
  if (button) button.disabled = !enabled;
}

async function loadBalance() {
  try {
    const data = await apiGet("/api/wallet/balance");
    document.getElementById("balance").textContent = formatCents(data.balance_cents);
  } catch (error) {
    setMessage(error.message, "error");
  }
}

async function deposit(event) {
  event.preventDefault();
  const form = event.target;
  const amountCents = dollarsToCents(form.amount.value);
  if (!amountCents || amountCents <= 0) {
    setMessage("Monto inválido", "error");
    return;
  }
  try {
    const data = await apiPost("/api/wallet/deposit", { amount_cents: amountCents });
    document.getElementById("balance").textContent = formatCents(data.new_balance_cents);
    setMessage("Depósito realizado", "success");
    form.reset();
  } catch (error) {
    setMessage(error.message, "error");
  }
}

async function newHand(event) {
  event.preventDefault();
  const form = event.target;
  const betCents = dollarsToCents(form.bet.value);
  if (!betCents || betCents <= 0) {
    setMessage("Apuesta inválida", "error");
    return;
  }
  try {
    const data = await apiPost("/api/games/blackjack/hands", { bet_amount_cents: betCents });
    handId = data.hand_id;
    currentHand = {
      sub_hands: data.sub_hands || [],
      dealer_cards: data.dealer_visible_card ? [data.dealer_visible_card] : [],
      status: data.status,
      total_win_amount_cents: 0,
    };
    selectedSubHand = firstActiveIndex(currentHand.sub_hands);
    render();
    if (isSettled(currentHand.status)) {
      setMessage("Mano resuelta", "success");
      loadBalance();
    } else {
      setMessage("");
    }
  } catch (error) {
    setMessage(error.message, "error");
  }
}

async function doAction(action) {
  if (!handId) {
    setMessage("No hay mano activa", "error");
    return;
  }
  try {
    const data = await apiPost("/api/games/blackjack/hands/" + handId + "/action", {
      sub_hand_index: selectedSubHand,
      action: action,
    });
    currentHand = {
      sub_hands: data.sub_hands || [],
      dealer_cards: data.dealer_cards || [],
      status: data.status,
      total_win_amount_cents: data.total_win_amount_cents || 0,
    };
    selectedSubHand = firstActiveIndex(currentHand.sub_hands);
    render();
    if (isSettled(currentHand.status)) {
      const totalBet = (currentHand.sub_hands || []).reduce(
        (sum, subHand) => sum + (subHand.bet_amount_cents || 0),
        0
      );
      const net = (currentHand.total_win_amount_cents || 0) - totalBet;
      if (net > 0) {
        setMessage("Ganaste " + formatCents(net), "success");
      } else if (net < 0) {
        setMessage("Perdiste " + formatCents(-net), "error");
      } else {
        setMessage("Empate", "");
      }
      loadBalance();
    }
  } catch (error) {
    setMessage(error.message, "error");
  }
}

function render() {
  const dealerElement = document.getElementById("dealer-cards");
  const playerElement = document.getElementById("player-hands");
  const selectorElement = document.getElementById("subhand-selector");

  if (!currentHand) {
    dealerElement.innerHTML = "";
    document.getElementById("dealer-total").textContent = "";
    playerElement.innerHTML =
      '<p class="muted">Sin mano en juego. Ingresa una apuesta y pulsa "Nueva mano".</p>';
    selectorElement.innerHTML = "";
    setActionsEnabled(false);
    return;
  }

  const dealerCards = currentHand.dealer_cards || [];
  dealerElement.innerHTML = dealerCards.map(cardHtml).join("");
  const dealerHasHidden = dealerCards.some((card) => card.hidden);
  document.getElementById("dealer-total").textContent =
    !dealerHasHidden && dealerCards.length ? "Total: " + handTotal(dealerCards) : "";

  const subHands = currentHand.sub_hands || [];
  playerElement.innerHTML = subHands
    .map((subHand, index) => {
      const cards = (subHand.cards || []).map(cardHtml).join("");
      const resultText = subHand.result
        ? '<span class="result ' + subHand.result + '">' + subHand.result + "</span>"
        : "";
      return (
        '<div class="subhand ' +
        (index === selectedSubHand ? "selected" : "") +
        '"><div class="subhand-header"><strong>Mano ' +
        subHand.index +
        "</strong> · " +
        formatCents(subHand.bet_amount_cents) +
        ' <span class="status">' +
        (subHand.status || "") +
        "</span> " +
        resultText +
        '</div><div class="cards">' +
        cards +
        '</div><div class="hand-total">Total: ' +
        handTotal(subHand.cards) +
        "</div></div>"
      );
    })
    .join("");

  if (subHands.length > 1) {
    selectorElement.innerHTML = subHands
      .map(
        (subHand, index) =>
          '<button class="btn subhand-btn ' +
          (index === selectedSubHand ? "active" : "") +
          '" data-index="' +
          index +
          '">Mano ' +
          subHand.index +
          "</button>"
      )
      .join("");
    selectorElement.querySelectorAll(".subhand-btn").forEach((button) => {
      button.addEventListener("click", () => {
        selectedSubHand = Number(button.dataset.index);
        render();
      });
    });
  } else {
    selectorElement.innerHTML = "";
  }

  const active = subHands[selectedSubHand];
  const handActive = currentHand.status === "active";
  const canAct = handActive && active && active.status === "active";
  const twoCards = active && active.cards && active.cards.length === 2;
  const canSplit =
    canAct &&
    subHands.length === 1 &&
    twoCards &&
    active.cards[0].rank === active.cards[1].rank;

  setAction("hit", canAct);
  setAction("stand", canAct);
  setAction("double", canAct && twoCards);
  setAction("split", canSplit);
  setAction("surrender", canAct && subHands.length === 1);
}

document.addEventListener("DOMContentLoaded", () => {
  if (!requireAuth()) return;

  renderNav("game.html");
  loadBalance();

  document.getElementById("deposit-form").addEventListener("submit", deposit);
  document.getElementById("bet-form").addEventListener("submit", newHand);
  document.querySelectorAll(".actions button").forEach((button) => {
    button.addEventListener("click", () => doAction(button.dataset.action));
  });

  setActionsEnabled(false);
  render();
});
