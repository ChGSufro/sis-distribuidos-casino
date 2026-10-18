function requireAuth() {
  if (!isLoggedIn()) {
    window.location.href = "index.html";
    return false;
  }
  return true;
}

async function logout() {
  try {
    if (isLoggedIn()) await apiRequest("POST", "/api/auth/logout");
  } catch (e) {
    // la sesión se limpia igual
  }
  clearSession();
  window.location.href = "index.html";
}

function renderNav(active) {
  const container = document.getElementById("app-nav");
  if (!container) return;

  const username = localStorage.getItem("username") || "";
  const links = [
    { href: "game.html", label: "Jugar" },
    { href: "history.html", label: "Historial" },
    { href: "stats.html", label: "Estadísticas" },
  ];

  const linksHtml = links
    .map(
      (link) =>
        '<a class="' +
        (link.href === active ? "active" : "") +
        '" href="' +
        link.href +
        '">' +
        link.label +
        "</a>"
    )
    .join("");

  container.innerHTML =
    '<span class="brand">Casino</span>' +
    '<nav class="nav-links">' +
    linksHtml +
    "</nav>" +
    '<span class="nav-user">' +
    (username ? "<span>" + username + "</span>" : "") +
    '<button id="logout-btn" class="btn-link">Salir</button>' +
    "</span>";

  const button = document.getElementById("logout-btn");
  if (button) button.addEventListener("click", logout);
}
