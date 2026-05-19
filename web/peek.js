// peek annotation sidebar — fetches /annotations, renders the sidebar,
// and wires per-block markers + inline note-input forms. Also keeps a
// heartbeat alive so the server can shut itself down when the tab closes.

const HASH_PREFIX = 'peek-';
const MAX_ANCHOR_TEXT = 200;
const HEARTBEAT_INTERVAL_MS = 5000;

async function loadNotes() {
  const res = await fetch('/annotations');
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const data = await res.json();
  return data.notes || [];
}

// Build hash -> section-heading map by walking blocks in document order.
function indexSections() {
  const map = new Map();
  let current = null;
  for (const block of document.querySelectorAll('[data-peek-block]')) {
    const heading = block.querySelector(':scope > h1, :scope > h2, :scope > h3, :scope > h4, :scope > h5, :scope > h6');
    if (heading) current = heading.textContent.trim();
    if (!block.id.startsWith(HASH_PREFIX)) continue;
    const hash = block.id.slice(HASH_PREFIX.length);
    map.set(hash, current);
  }
  return map;
}

function groupBySection(notes, sections) {
  const groups = new Map();
  for (const note of notes) {
    const hash = note.anchor?.block_hash;
    const section = hash && sections.has(hash) ? sections.get(hash) : null;
    const key = section ?? '(unsectioned)';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(note);
  }
  return groups;
}

function renderNote(note) {
  const li = document.createElement('li');
  li.className = 'peek-note' + (note.resolved ? ' resolved' : '');
  li.dataset.noteId = note.id;

  const anchor = document.createElement('div');
  anchor.className = 'peek-note-anchor';
  anchor.textContent = note.anchor?.block_text || '';
  anchor.title = anchor.textContent;

  const body = document.createElement('div');
  body.className = 'peek-note-body';
  body.textContent = note.body;

  const time = document.createElement('time');
  time.className = 'peek-note-time';
  if (note.created_at) {
    time.dateTime = note.created_at;
    try {
      time.textContent = new Date(note.created_at).toLocaleString();
    } catch {
      time.textContent = note.created_at;
    }
  }

  const controls = document.createElement('div');
  controls.className = 'peek-note-controls';

  const resolvedLabel = document.createElement('label');
  resolvedLabel.className = 'peek-note-resolved';
  resolvedLabel.title = 'Mark as resolved';
  const checkbox = document.createElement('input');
  checkbox.type = 'checkbox';
  checkbox.checked = !!note.resolved;
  checkbox.addEventListener('change', () => {
    updateNote(note.id, { resolved: checkbox.checked }).catch(() => {
      checkbox.checked = !checkbox.checked;
    });
  });
  const resolvedText = document.createElement('span');
  resolvedText.textContent = 'resolved';
  resolvedLabel.append(checkbox, resolvedText);

  const editBtn = document.createElement('button');
  editBtn.type = 'button';
  editBtn.className = 'peek-note-edit';
  editBtn.textContent = 'edit';
  editBtn.addEventListener('click', () => openEditNote(note, li));

  const deleteBtn = document.createElement('button');
  deleteBtn.type = 'button';
  deleteBtn.className = 'peek-note-delete';
  deleteBtn.textContent = 'delete';
  wireConfirmDelete(deleteBtn, note.id);

  controls.append(resolvedLabel, editBtn, deleteBtn);

  li.append(anchor, body, time, controls);
  return li;
}

function wireConfirmDelete(btn, noteId) {
  let armed = false;
  let timer = null;
  const reset = () => {
    armed = false;
    btn.textContent = 'delete';
    btn.classList.remove('confirm');
    if (timer) { clearTimeout(timer); timer = null; }
  };
  btn.addEventListener('click', async (e) => {
    e.stopPropagation();
    if (!armed) {
      armed = true;
      btn.textContent = 'confirm?';
      btn.classList.add('confirm');
      timer = setTimeout(reset, 3000);
      return;
    }
    reset();
    try {
      await deleteNote(noteId);
    } catch (err) {
      btn.title = err.message;
    }
  });
  btn.addEventListener('blur', reset);
}

function openEditNote(note, li) {
  closeAllForms();
  for (const el of document.querySelectorAll('.peek-note.editing')) {
    el.classList.remove('editing');
    const f = el.querySelector('.peek-note-edit-form');
    if (f) f.remove();
  }
  li.classList.add('editing');

  const form = document.createElement('form');
  form.className = 'peek-form peek-note-edit-form';
  form.innerHTML = `
    <textarea class="peek-form-textarea" rows="3" required></textarea>
    <div class="peek-form-error" hidden></div>
    <div class="peek-form-actions">
      <span class="peek-form-hint">⌘/Ctrl+Enter to save · Esc to cancel</span>
      <button type="button" class="peek-form-cancel">Cancel</button>
      <button type="submit" class="peek-form-save">Save</button>
    </div>
  `;
  const ta = form.querySelector('.peek-form-textarea');
  ta.value = note.body;
  li.querySelector('.peek-note-body').after(form);
  ta.focus();
  ta.setSelectionRange(ta.value.length, ta.value.length);

  const close = () => {
    li.classList.remove('editing');
    form.remove();
  };

  form.querySelector('.peek-form-cancel').addEventListener('click', close);
  ta.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') { e.preventDefault(); close(); }
    else if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); form.requestSubmit(); }
  });
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const body = ta.value.trim();
    if (!body) return;
    const saveBtn = form.querySelector('.peek-form-save');
    const errEl = form.querySelector('.peek-form-error');
    errEl.hidden = true;
    saveBtn.disabled = true;
    try {
      await updateNote(note.id, { body });
      close();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.hidden = false;
      saveBtn.disabled = false;
    }
  });
}

async function updateNote(id, patch) {
  const res = await fetch(`/annotations/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(patch),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  await refresh();
}

async function deleteNote(id) {
  const res = await fetch(`/annotations/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  await refresh();
}

function renderSidebar(notes) {
  const container = document.getElementById('peek-notes');
  if (!container) return;
  container.replaceChildren();

  if (notes.length === 0) {
    const p = document.createElement('p');
    p.className = 'peek-empty';
    p.textContent = 'No notes yet.';
    container.appendChild(p);
    return;
  }

  const sections = indexSections();
  const groups = groupBySection(notes, sections);

  for (const [section, group] of groups) {
    const h3 = document.createElement('h3');
    h3.className = 'peek-section';
    h3.textContent = section;
    container.appendChild(h3);

    const ul = document.createElement('ul');
    ul.className = 'peek-notes-list';
    for (const note of group) ul.appendChild(renderNote(note));
    container.appendChild(ul);
  }
}

function renderError(message) {
  const container = document.getElementById('peek-notes');
  if (!container) return;
  container.replaceChildren();
  const p = document.createElement('p');
  p.className = 'peek-error';
  p.textContent = message;
  container.appendChild(p);
}

function injectMarkers() {
  for (const block of document.querySelectorAll('[data-peek-block]')) {
    if (block.querySelector(':scope > .peek-marker')) continue;
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'peek-marker';
    btn.setAttribute('aria-label', 'Add note');
    btn.textContent = '+';
    block.appendChild(btn);
  }
}

function updateMarkers(notes) {
  const annotated = new Set();
  for (const n of notes) annotated.add(n.anchor?.block_hash);
  for (const block of document.querySelectorAll('[data-peek-block]')) {
    const hash = block.id.slice(HASH_PREFIX.length);
    const marker = block.querySelector(':scope > .peek-marker');
    if (!marker) continue;
    const has = annotated.has(hash);
    marker.classList.toggle('has-notes', has);
    marker.setAttribute('aria-label', has ? 'Add another note' : 'Add note');
  }
}

function detectBlockType(block) {
  const first = block.firstElementChild;
  if (!first) return 'block';
  const tag = first.tagName.toLowerCase();
  if (/^h[1-6]$/.test(tag)) return 'heading';
  if (tag === 'p') return 'paragraph';
  if (tag === 'ul' || tag === 'ol') return 'list';
  if (tag === 'blockquote') return 'blockquote';
  if (tag === 'pre') return 'code';
  if (tag === 'div' && first.classList.contains('mermaid')) return 'mermaid';
  return tag;
}

function blockText(block) {
  const clone = block.cloneNode(true);
  for (const el of clone.querySelectorAll('.peek-marker, .peek-form')) el.remove();
  const text = clone.textContent.replace(/\s+/g, ' ').trim();
  return text.length > MAX_ANCHOR_TEXT ? text.slice(0, MAX_ANCHOR_TEXT) + '…' : text;
}

function closeAllForms() {
  for (const f of document.querySelectorAll('.peek-form')) f.remove();
}

function openForm(block) {
  closeAllForms();

  const form = document.createElement('form');
  form.className = 'peek-form';
  form.innerHTML = `
    <textarea class="peek-form-textarea" placeholder="Add a note…" required rows="3"></textarea>
    <div class="peek-form-error" hidden></div>
    <div class="peek-form-actions">
      <span class="peek-form-hint">⌘/Ctrl+Enter to save · Esc to cancel</span>
      <button type="button" class="peek-form-cancel">Cancel</button>
      <button type="submit" class="peek-form-save">Save</button>
    </div>
  `;
  block.after(form);

  const ta = form.querySelector('.peek-form-textarea');
  ta.focus();

  form.querySelector('.peek-form-cancel').addEventListener('click', () => form.remove());
  ta.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      form.remove();
    } else if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      form.requestSubmit();
    }
  });

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const body = ta.value.trim();
    if (!body) return;
    const saveBtn = form.querySelector('.peek-form-save');
    const errEl = form.querySelector('.peek-form-error');
    errEl.hidden = true;
    saveBtn.disabled = true;
    try {
      await createNote(block, body);
      form.remove();
      await refresh();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.hidden = false;
      saveBtn.disabled = false;
    }
  });
}

async function createNote(block, body) {
  const res = await fetch('/annotations', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      anchor: {
        block_hash: block.id.slice(HASH_PREFIX.length),
        block_text: blockText(block),
        block_type: detectBlockType(block),
      },
      body,
    }),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
}

function wireMarkerClicks() {
  document.addEventListener('click', (e) => {
    const marker = e.target.closest('.peek-marker');
    if (!marker) return;
    const block = marker.closest('[data-peek-block]');
    if (!block) return;
    e.preventDefault();
    e.stopPropagation();
    openForm(block);
  });
}

function wireCollapse() {
  const btn = document.getElementById('peek-collapse');
  btn?.addEventListener('click', () => {
    document.body.classList.toggle('peek-collapsed');
    btn.textContent = document.body.classList.contains('peek-collapsed') ? '‹' : '›';
  });
}

async function refresh() {
  const notes = await loadNotes();
  renderSidebar(notes);
  updateMarkers(notes);
}

function wireLifecycle() {
  const beat = () => {
    fetch('/heartbeat', { method: 'POST', keepalive: true }).catch(() => {});
  };
  beat();
  setInterval(beat, HEARTBEAT_INTERVAL_MS);
  addEventListener('pagehide', () => {
    navigator.sendBeacon('/bye');
  });
}

async function init() {
  wireLifecycle();
  wireCollapse();
  injectMarkers();
  wireMarkerClicks();
  try {
    await refresh();
  } catch (err) {
    renderError(`Failed to load notes: ${err.message}`);
  }
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
