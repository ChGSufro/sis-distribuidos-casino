function getToken() {
  return localStorage.getItem("token");
}

function getRefreshToken() {
  return localStorage.getItem("refresh_token");
}

function isLoggedIn() {
  return !!getToken();
}

function setSession(data) {
  if (!data) return;
  if (data.access_token) localStorage.setItem("token", data.access_token);
  if (data.refresh_token) localStorage.setItem("refresh_token", data.refresh_token);
  if (data.user_id) localStorage.setItem("user_id", data.user_id);
  if (data.username) localStorage.setItem("username", data.username);
}

function clearSession() {
  localStorage.removeItem("token");
  localStorage.removeItem("refresh_token");
  localStorage.removeItem("user_id");
  localStorage.removeItem("username");
}

async function rawRequest(method, path, body, token) {
  const headers = { "Content-Type": "application/json" };
  if (token) headers["Authorization"] = "Bearer " + token;
  const options = { method, headers };
  if (body !== undefined) options.body = JSON.stringify(body);
  const response = await fetch(API_BASE + path, options);
  let json = null;
  try {
    json = await response.json();
  } catch (e) {
    json = null;
  }
  return { status: response.status, json };
}

async function tryRefresh() {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return false;
  const { status, json } = await rawRequest(
    "POST",
    "/api/auth/refresh",
    { refresh_token: refreshToken },
    null
  );
  if (status >= 200 && status < 300 && json && json.success && json.data) {
    setSession(json.data);
    return true;
  }
  return false;
}

async function apiRequest(method, path, options) {
  options = options || {};
  const auth = options.auth !== false;
  const body = options.body;

  let result = await rawRequest(method, path, body, auth ? getToken() : null);

  if (result.status === 401 && auth) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      result = await rawRequest(method, path, body, getToken());
    }
  }

  if (result.status === 401) {
    clearSession();
    window.location.href = "index.html";
    throw new Error("Sesión expirada");
  }

  if (!result.json) {
    throw new Error("Respuesta inválida del servidor");
  }

  if (!result.json.success) {
    throw new Error(result.json.error || "Error del servidor (" + result.status + ")");
  }

  return result.json.data;
}

function apiGet(path) {
  return apiRequest("GET", path);
}

function apiPost(path, body) {
  return apiRequest("POST", path, { body });
}

function formatCents(cents) {
  const value = Number(cents || 0) / 100;
  return "$" + value.toFixed(2);
}
