function showMessage(text, type) {
  const element = document.getElementById("auth-message");
  element.textContent = text || "";
  element.className = "message" + (type ? " " + type : "");
}

function switchTab(tab) {
  document.querySelectorAll(".tab").forEach((button) => {
    button.classList.toggle("active", button.dataset.tab === tab);
  });
  document.querySelectorAll(".tab-panel").forEach((panel) => {
    panel.classList.toggle("active", panel.id === tab + "-tab");
  });
}

document.addEventListener("DOMContentLoaded", () => {
  if (isLoggedIn()) {
    window.location.href = "game.html";
    return;
  }

  document.querySelectorAll(".tab").forEach((button) => {
    button.addEventListener("click", () => {
      switchTab(button.dataset.tab);
      showMessage("", "");
    });
  });

  document.getElementById("login-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.target;
    try {
      const data = await apiRequest("POST", "/api/auth/login", {
        auth: false,
        body: { email: form.email.value.trim(), password: form.password.value },
      });
      setSession(data);
      window.location.href = "game.html";
    } catch (error) {
      showMessage(error.message, "error");
    }
  });

  document.getElementById("register-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.target;
    try {
      await apiRequest("POST", "/api/auth/register", {
        auth: false,
        body: {
          username: form.username.value.trim(),
          email: form.email.value.trim(),
          password: form.password.value,
        },
      });
      switchTab("login");
      showMessage("Cuenta creada. Ya puedes iniciar sesión.", "success");
    } catch (error) {
      showMessage(error.message, "error");
    }
  });
});
