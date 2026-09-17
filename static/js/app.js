(function () {
  'use strict';

  const isAdmin = document.body.dataset.admin === 'true';
  const $ = (id) => document.getElementById(id);

  let selectedDate = null;
  let calendarDirty = false;

  /* ---------- Theme ---------- */

  const THEME_COLORS = { light: '#ffffff', dark: '#12151c' };

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute('content', THEME_COLORS[theme]);
  }

  function storedTheme() {
    try { return localStorage.getItem('theme'); } catch (e) { return null; }
  }

  $('theme-toggle').addEventListener('click', () => {
    const next = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
    applyTheme(next);
    try { localStorage.setItem('theme', next); } catch (e) { /* storage unavailable */ }
  });

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    if (!storedTheme()) applyTheme(e.matches ? 'dark' : 'light');
  });

  /* ---------- Toasts ---------- */

  const TOAST_ICONS = {
    success: '<path d="M20 6L9 17l-5-5"/>',
    error: '<circle cx="12" cy="12" r="9"/><path d="M15 9l-6 6M9 9l6 6"/>',
    info: '<circle cx="12" cy="12" r="9"/><path d="M12 16v-4M12 8h.01"/>',
  };

  function toast(message, type = 'info') {
    const el = document.createElement('div');
    el.className = `toast toast-${type}`;
    el.setAttribute('role', type === 'error' ? 'alert' : 'status');
    el.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${TOAST_ICONS[type]}</svg><span></span>`;
    el.querySelector('span').textContent = message;
    $('toasts').appendChild(el);

    setTimeout(() => {
      el.classList.add('is-leaving');
      setTimeout(() => el.remove(), 250);
    }, 3500);
  }

  /* ---------- API ---------- */

  async function api(url, options = {}) {
    if (options.body && typeof options.body !== 'string') {
      options.headers = { 'Content-Type': 'application/json', ...options.headers };
      options.body = JSON.stringify(options.body);
    }
    let res;
    try {
      res = await fetch(url, { credentials: 'same-origin', ...options });
    } catch (err) {
      throw new Error(navigator.onLine ? 'Could not reach the server' : 'You are offline');
    }
    let data = {};
    try { data = await res.json(); } catch (e) { /* non-JSON response */ }
    if (!res.ok || !data.success) {
      const error = new Error(data.message || `Request failed (${res.status})`);
      error.status = res.status;
      throw error;
    }
    return data;
  }

  async function withBusy(form, fn) {
    const submit = form.querySelector('[type="submit"]');
    if (submit) submit.disabled = true;
    try {
      await fn();
    } finally {
      if (submit) submit.disabled = false;
    }
  }

  /* ---------- Dialogs ---------- */

  function openDialog(dialog) {
    if (!dialog.open) dialog.showModal();
  }

  document.querySelectorAll('dialog').forEach((dialog) => {
    dialog.addEventListener('click', (e) => {
      // Click on the backdrop (outside the dialog box) closes it
      if (e.target === dialog) dialog.close();
      if (e.target.closest('[data-close]')) dialog.close();
    });
  });

  /* ---------- Day dialog ---------- */

  const dayDialog = $('day-dialog');

  function formatDate(dateStr) {
    return new Date(dateStr + 'T00:00:00').toLocaleDateString('en-US', {
      weekday: 'long', month: 'long', day: 'numeric', year: 'numeric',
    });
  }

  function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text != null) node.textContent = text;
    return node;
  }

  function renderTasks(tasks) {
    const list = $('day-tasks');
    list.replaceChildren();

    tasks.forEach((t) => {
      const card = el('article', `task ${t.is_booked ? 'is-booked' : 'is-open'}`);
      const main = el('div', 'task-main');

      const meta = el('div', 'task-meta');
      meta.append(
        el('span', 'task-time', `${t.start_time} – ${t.end_time}`),
        el('span', `badge ${t.is_booked ? 'badge-danger' : 'badge-success'}`, t.is_booked ? 'Booked' : 'Available'),
      );
      main.append(meta, el('h3', 'task-title', t.title));

      if (t.description) main.append(el('p', 'task-desc', t.description));
      if (t.requested_by_name) {
        const who = t.requested_by_email ? `${t.requested_by_name} · ${t.requested_by_email}` : t.requested_by_name;
        main.append(el('p', 'task-requester', `Requested by ${who}`));
      }
      card.append(main);

      if (isAdmin) {
        const actions = el('div', 'task-actions');
        const toggle = el('button', 'btn btn-sm', t.is_booked ? 'Set available' : 'Set booked');
        toggle.type = 'button';
        toggle.addEventListener('click', () => toggleTask(t.id));

        const del = el('button', 'btn btn-sm btn-danger', 'Delete');
        del.type = 'button';
        del.addEventListener('click', () => deleteTask(t.id, t.title));

        actions.append(toggle, del);
        card.append(actions);
      }

      list.append(card);
    });
  }

  async function loadDay(dateStr) {
    $('day-loading').hidden = false;
    $('day-empty').hidden = true;
    $('day-tasks').replaceChildren();
    $('day-dialog-sub').textContent = 'Loading…';

    try {
      const { data } = await api(`/api/tasks?date=${encodeURIComponent(dateStr)}`);
      const tasks = data.tasks || [];
      const open = tasks.filter((t) => !t.is_booked).length;
      const booked = tasks.length - open;

      $('day-dialog-sub').textContent = tasks.length
        ? `${open} available · ${booked} booked`
        : 'Nothing scheduled';
      $('day-empty').hidden = tasks.length > 0;
      renderTasks(tasks);
    } catch (err) {
      $('day-dialog-sub').textContent = 'Could not load schedule';
      toast(err.message, 'error');
    } finally {
      $('day-loading').hidden = true;
    }
  }

  function openDay(dateStr) {
    selectedDate = dateStr;
    $('day-dialog-title').textContent = formatDate(dateStr);
    openDialog(dayDialog);
    loadDay(dateStr);
  }

  dayDialog.addEventListener('close', () => {
    // Refresh the month grid so status colors reflect changes
    if (calendarDirty) window.location.reload();
  });

  document.addEventListener('click', (e) => {
    const trigger = e.target.closest('[data-date]');
    if (trigger) openDay(trigger.dataset.date);
  });

  async function toggleTask(id) {
    try {
      await api(`/api/tasks/${id}/toggle`, { method: 'PUT' });
      calendarDirty = true;
      toast('Status updated', 'success');
      loadDay(selectedDate);
    } catch (err) {
      handleAdminError(err);
    }
  }

  async function deleteTask(id, title) {
    if (!confirm(`Delete "${title}"?`)) return;
    try {
      await api(`/api/tasks/${id}`, { method: 'DELETE' });
      calendarDirty = true;
      toast('Entry deleted', 'info');
      loadDay(selectedDate);
    } catch (err) {
      handleAdminError(err);
    }
  }

  function handleAdminError(err) {
    if (err.status === 401) {
      toast('Your admin session expired. Please sign in again.', 'error');
      dayDialog.close();
      openDialog($('login-dialog'));
      return;
    }
    toast(err.message, 'error');
  }

  function formValues(form) {
    return Object.fromEntries(new FormData(form).entries());
  }

  function checkTimes(values) {
    if (values.end_time <= values.start_time) {
      toast('End time must be after start time', 'error');
      return false;
    }
    return true;
  }

  const requestForm = $('request-form');
  if (requestForm) {
    requestForm.addEventListener('submit', (e) => {
      e.preventDefault();
      const values = formValues(requestForm);
      if (!checkTimes(values)) return;

      withBusy(requestForm, async () => {
        try {
          await api('/api/request-slot', { method: 'POST', body: { ...values, task_date: selectedDate } });
          calendarDirty = true;
          toast('Request sent. You will be contacted by email.', 'success');
          ['req-title', 'req-description'].forEach((id) => { $(id).value = ''; });
          loadDay(selectedDate);
        } catch (err) {
          toast(err.message, 'error');
        }
      });
    });
  }

  const createForm = $('create-task-form');
  if (createForm) {
    createForm.addEventListener('submit', (e) => {
      e.preventDefault();
      const values = formValues(createForm);
      if (!checkTimes(values)) return;

      withBusy(createForm, async () => {
        try {
          await api('/api/tasks', {
            method: 'POST',
            body: { ...values, is_booked: $('task-booked').checked, task_date: selectedDate },
          });
          calendarDirty = true;
          toast('Entry added', 'success');
          ['task-title', 'task-description'].forEach((id) => { $(id).value = ''; });
          $('task-booked').checked = false;
          loadDay(selectedDate);
        } catch (err) {
          handleAdminError(err);
        }
      });
    });
  }

  /* ---------- Admin auth ---------- */

  const loginBtn = $('login-btn');
  if (loginBtn) {
    loginBtn.addEventListener('click', () => {
      openDialog($('login-dialog'));
      $('admin-password').focus();
    });
  }

  $('toggle-password').addEventListener('click', (e) => {
    const input = $('admin-password');
    const show = input.type === 'password';
    input.type = show ? 'text' : 'password';
    e.currentTarget.setAttribute('aria-pressed', String(show));
    e.currentTarget.setAttribute('aria-label', show ? 'Hide password' : 'Show password');
  });

  const loginForm = $('login-form');
  loginForm.addEventListener('submit', (e) => {
    e.preventDefault();
    withBusy(loginForm, async () => {
      try {
        await api('/api/login', { method: 'POST', body: { password: $('admin-password').value } });
        toast('Signed in as admin', 'success');
        setTimeout(() => window.location.reload(), 400);
      } catch (err) {
        toast(err.message, 'error');
        $('admin-password').select();
      }
    });
  });

  const logoutBtn = $('logout-btn');
  if (logoutBtn) {
    logoutBtn.addEventListener('click', async () => {
      try {
        await api('/api/logout', { method: 'POST' });
        window.location.reload();
      } catch (err) {
        toast(err.message, 'error');
      }
    });
  }

  /* ---------- Deep link: /?date=YYYY-MM-DD or ?date=today ---------- */

  const params = new URLSearchParams(window.location.search);
  const dateParam = params.get('date');
  if (dateParam) {
    const d = new Date();
    const today = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
    const target = dateParam === 'today' ? today : dateParam;
    if (/^\d{4}-\d{2}-\d{2}$/.test(target)) openDay(target);
  }

  /* ---------- PWA: offline state, service worker, install ---------- */

  function updateOnlineState() {
    $('offline-banner').hidden = navigator.onLine;
  }
  window.addEventListener('online', updateOnlineState);
  window.addEventListener('offline', updateOnlineState);
  updateOnlineState();

  if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
      navigator.serviceWorker.register('/sw.js').catch((err) => console.warn('Service worker registration failed', err));
    });
  }

  let installPrompt = null;
  const installBtn = $('install-btn');

  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault();
    installPrompt = e;
    installBtn.hidden = false;
  });

  installBtn.addEventListener('click', async () => {
    if (!installPrompt) return;
    installPrompt.prompt();
    await installPrompt.userChoice;
    installPrompt = null;
    installBtn.hidden = true;
  });

  window.addEventListener('appinstalled', () => {
    installBtn.hidden = true;
    toast('ChronosHub installed', 'success');
  });
})();
