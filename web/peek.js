// peek annotation sidebar — fetches /annotations and renders notes grouped
// by their nearest preceding heading. Read-only for now; mutation handlers
// land in later steps.

const HASH_PREFIX = 'peek-';

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

  li.append(anchor, body, time);
  return li;
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

function wireCollapse() {
  const btn = document.getElementById('peek-collapse');
  btn?.addEventListener('click', () => {
    document.body.classList.toggle('peek-collapsed');
    btn.textContent = document.body.classList.contains('peek-collapsed') ? '‹' : '›';
  });
}

async function init() {
  wireCollapse();
  try {
    const notes = await loadNotes();
    renderSidebar(notes);
  } catch (err) {
    renderError(`Failed to load notes: ${err.message}`);
  }
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
