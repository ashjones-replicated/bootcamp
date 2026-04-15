'use strict';

const IMAGE_EXTS = new Set(['jpg','jpeg','png','gif','webp','svg','bmp','avif']);

function filePreview(f) {
    const ext = f.name.split('.').pop().toLowerCase();
    if (IMAGE_EXTS.has(ext)) {
        return `<a href="${escHtml(f.url)}" target="_blank">
            <img class="preview-img" src="${escHtml(f.url)}" alt="" loading="lazy">
        </a>`;
    }
    return `<div class="preview-type">${escHtml(ext)}</div>`;
}

function escHtml(s) {
    return String(s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function timeAgo(unixSec) {
    const s = Math.floor(Date.now() / 1000) - unixSec;
    if (s < 60)    return 'just now';
    if (s < 3600)  return Math.floor(s / 60) + 'm ago';
    if (s < 86400) return Math.floor(s / 3600) + 'h ago';
    return Math.floor(s / 86400) + 'd ago';
}

/* ── Update banner ──────────────────────────────────── */

const updateBanner = document.getElementById('update-banner');
if (updateBanner) {
    fetch('/api/updates')
        .then(r => r.json())
        .then(data => {
            if (!data.available) return;
            updateBanner.querySelector('.update-banner-msg').textContent =
                data.version ? `Update available: v${data.version}` : 'An update is available.';
            updateBanner.hidden = false;
        })
        .catch(() => {});
}

/* ── Index page ─────────────────────────────────────── */

let currentUsername = '';

const uploadForm = document.getElementById('upload-form');
if (uploadForm) {
    fetch('/api/me')
        .then(r => r.json())
        .then(me => {
            currentUsername = me.username;
            const display = document.getElementById('username-display');
            if (display) display.textContent = me.username;
            if (me.is_admin) {
                const nav = document.querySelector('.header-nav');
                const link = document.createElement('a');
                link.href = '/admin';
                link.className = 'btn btn-secondary btn-sm';
                link.textContent = 'Admin';
                nav.insertBefore(link, nav.firstChild);
            }
            loadFiles();
        })
        .catch(() => {});

    uploadForm.addEventListener('submit', async e => {
        e.preventDefault();
        const status = document.getElementById('upload-status');
        const fileInput = document.getElementById('file-input');

        if (!fileInput.files[0]) {
            status.textContent = 'Please select a file.';
            status.className = 'status-msg err';
            return;
        }

        const data = new FormData();
        data.append('file', fileInput.files[0]);
        if (document.getElementById('opt-private').checked) data.append('private', 'on');
        if (document.getElementById('opt-single').checked)  data.append('single', 'on');

        status.textContent = 'Uploading\u2026';
        status.className = 'status-msg';

        try {
            const resp = await fetch('/upload', { method: 'POST', body: data });
            if (!resp.ok) throw new Error(await resp.text());
            const result = await resp.json();
            status.textContent = 'Uploaded: ' + result.name;
            status.className = 'status-msg ok';
            fileInput.value = '';
            loadFiles();
        } catch (err) {
            status.textContent = 'Error: ' + err.message;
            status.className = 'status-msg err';
        }
    });
}

async function loadFiles() {
    const container = document.getElementById('file-list');
    if (!container) return;
    try {
        const resp = await fetch('/api/files');
        if (!resp.ok) throw new Error('request failed');
        const files = (await resp.json())
            .filter(f => !f.uploaded_by || f.uploaded_by === currentUsername);
        if (!files.length) {
            container.innerHTML = '<p class="empty">No files yet.</p>';
            return;
        }
        container.innerHTML = '<ul class="file-list">' +
            files.map(f =>
                `<li class="file-item">
                    ${filePreview(f)}
                    <div class="file-item-row">
                        <a href="${escHtml(f.url)}" target="_blank">${escHtml(f.name)}</a>
                        <button class="copy-btn" onclick="copyLink(this, '${escHtml(location.origin + f.url)}')">Copy</button>
                        <button class="copy-btn" onclick="deleteFile('${escHtml(f.name)}', loadFiles)">Del</button>
                    </div>
                    <div class="file-meta">${timeAgo(f.uploaded_at)}</div>
                </li>`
            ).join('') +
            '</ul>';
    } catch {
        container.innerHTML = '<p class="empty">Failed to load files.</p>';
    }
}

/* ── User menu dropdown ─────────────────────────────── */

const userMenuBtn = document.getElementById('user-menu-btn');
if (userMenuBtn) {
    const dropdown = document.getElementById('user-menu-dropdown');
    const changePwToggle = document.getElementById('change-pw-toggle');
    const passwordForm = document.getElementById('password-form');

    userMenuBtn.addEventListener('click', e => {
        e.stopPropagation();
        dropdown.hidden = !dropdown.hidden;
    });

    document.addEventListener('click', e => {
        if (!dropdown.hidden && !dropdown.contains(e.target)) {
            dropdown.hidden = true;
            passwordForm.hidden = true;
        }
    });

    changePwToggle.addEventListener('click', () => {
        passwordForm.hidden = !passwordForm.hidden;
    });

    passwordForm.addEventListener('submit', async e => {
        e.preventDefault();
        const status  = document.getElementById('pw-status');
        const newPw   = document.getElementById('pw-new').value;
        const confirm = document.getElementById('pw-confirm').value;

        if (newPw !== confirm) {
            status.textContent = 'Passwords do not match.';
            status.className = 'status-msg err';
            return;
        }

        status.textContent = '';
        try {
            const resp = await fetch('/api/password', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ current: document.getElementById('pw-current').value, new: newPw }),
            });
            if (!resp.ok) throw new Error(await resp.text());
            status.textContent = 'Password changed.';
            status.className = 'status-msg ok';
            passwordForm.reset();
        } catch (err) {
            status.textContent = 'Error: ' + err.message;
            status.className = 'status-msg err';
        }
    });
}

/* ── Admin page ─────────────────────────────────────── */

const createUserForm = document.getElementById('create-user-form');
if (createUserForm) {
    loadUsers();
    loadAdminFiles();

    const overlay   = document.getElementById('modal-overlay');
    const addBtn    = document.getElementById('add-user-btn');
    const closeBtn  = document.getElementById('modal-close');

    fetch('/api/admin/entitlements')
        .then(r => r.json())
        .then(e => {
            if (e.allow_user_creation) addBtn.style.removeProperty('display');
            if (e.sdk_available) {
                const card = document.getElementById('support-card');
                if (card) card.style.removeProperty('display');
            }
        })
        .catch(() => {});

    function openModal()  { overlay.hidden = false; document.getElementById('new-username').focus(); }
    function closeModal() { overlay.hidden = true; createUserForm.reset();
                            document.getElementById('create-ok').textContent = '';
                            document.getElementById('create-err').textContent = ''; }

    addBtn.addEventListener('click', openModal);
    closeBtn.addEventListener('click', closeModal);
    overlay.addEventListener('click', e => { if (e.target === overlay) closeModal(); });

    createUserForm.addEventListener('submit', async e => {
        e.preventDefault();
        const ok  = document.getElementById('create-ok');
        const err = document.getElementById('create-err');
        ok.textContent = '';
        err.textContent = '';

        const body = {
            username: document.getElementById('new-username').value.trim(),
            password: document.getElementById('new-password').value,
            is_admin: document.getElementById('new-is-admin').checked,
        };

        if (!body.username || !body.password) {
            err.textContent = 'Username and password are required.';
            return;
        }

        try {
            const resp = await fetch('/api/admin/users', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
            });
            if (!resp.ok) throw new Error(await resp.text());
            closeModal();
            loadUsers();
        } catch (e) {
            err.textContent = 'Error: ' + e.message;
        }
    });
}

async function loadUsers() {
    const tbody = document.getElementById('user-tbody');
    if (!tbody) return;
    try {
        const resp = await fetch('/api/admin/users');
        if (!resp.ok) throw new Error('request failed');
        const users = await resp.json();
        if (!users.length) {
            tbody.innerHTML = '<tr><td colspan="4" class="empty">No users.</td></tr>';
            return;
        }
        tbody.innerHTML = users.map(u => `
            <tr>
                <td>${escHtml(u.username)}</td>
                <td><span class="badge ${u.is_admin ? 'admin' : ''}">${u.is_admin ? 'admin' : 'user'}</span></td>
                <td>${new Date(u.created_at).toLocaleDateString()}</td>
                <td>
                    <button class="btn btn-danger btn-sm"
                        onclick="deleteUser(${u.id}, '${escHtml(u.username)}')">Delete</button>
                </td>
            </tr>`
        ).join('');
    } catch {
        tbody.innerHTML = '<tr><td colspan="4">Failed to load users.</td></tr>';
    }
}

async function copyLink(btn, url) {
    await navigator.clipboard.writeText(url);
    const orig = btn.textContent;
    btn.textContent = 'Copied!';
    setTimeout(() => { btn.textContent = orig; }, 1500);
}

async function loadAdminFiles() {
    const tbody = document.getElementById('files-tbody');
    if (!tbody) return;
    try {
        const resp = await fetch('/api/files');
        if (!resp.ok) throw new Error('request failed');
        const files = await resp.json();
        if (!files.length) {
            tbody.innerHTML = '<tr><td colspan="5" class="empty">No files.</td></tr>';
            return;
        }
        tbody.innerHTML = files.map(f => `
            <tr>
                <td class="preview-cell">${filePreview(f)}</td>
                <td><a href="${escHtml(f.url)}" target="_blank">${escHtml(f.name)}</a></td>
                <td>${f.uploaded_by ? escHtml(f.uploaded_by) : '&mdash;'}</td>
                <td>${timeAgo(f.uploaded_at)}</td>
                <td><button class="btn btn-danger btn-sm" onclick="deleteFile('${escHtml(f.name)}', loadAdminFiles)">Delete</button></td>
            </tr>`
        ).join('');
    } catch {
        tbody.innerHTML = '<tr><td colspan="5">Failed to load files.</td></tr>';
    }
}

async function deleteUser(id, username) {
    if (!confirm(`Delete user "${username}"? This cannot be undone.`)) return;
    try {
        const resp = await fetch('/api/admin/users/' + id, { method: 'DELETE' });
        if (!resp.ok) throw new Error(await resp.text());
        loadUsers();
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

/* ── Support bundle ─────────────────────────────────── */

const generateBundleBtn = document.getElementById('generate-bundle-btn');
if (generateBundleBtn) {
    generateBundleBtn.addEventListener('click', async () => {
        const status = document.getElementById('bundle-status');
        generateBundleBtn.disabled = true;
        status.textContent = 'Generating support bundle\u2026 this may take a minute.';
        status.className = 'status-msg';

        try {
            const resp = await fetch('/api/admin/supportbundle', { method: 'POST' });
            if (!resp.ok) throw new Error(await resp.text());
            const result = await resp.json();
            status.textContent = 'Bundle uploaded. ID: ' + result.bundleId;
            status.className = 'status-msg ok';
        } catch (err) {
            status.textContent = 'Error: ' + err.message;
            status.className = 'status-msg err';
        } finally {
            generateBundleBtn.disabled = false;
        }
    });
}

async function deleteFile(name, reload) {
    if (!confirm(`Delete "${name}"? This cannot be undone.`)) return;
    try {
        const resp = await fetch('/files/' + encodeURIComponent(name), { method: 'DELETE' });
        if (!resp.ok) throw new Error(await resp.text());
        reload();
    } catch (e) {
        alert('Error: ' + e.message);
    }
}
