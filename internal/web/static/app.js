/**
 * TeleDrive Modern Frontend Engine
 * Zero-Build, High-Performance, Accessible
 */

// --- Global State ---
let currentFolderId = null;
let currentFolderPath = [];
let cachedFolders = [];
let cachedFiles = [];
let currentView = localStorage.getItem("teledrive_view") || "grid";
let currentSort = { field: "name", order: "asc" };
let currentTab = "drive"; // "drive" or "shares"
let searchDebounceTimer = null;
let activeSearchQuery = "";
let selectedItems = new Map(); // key: "type:id" -> { id, type, name }
let draggedItem = null; // { id, type }

// Upload Manager State
let uploadQueue = [];
let isUploading = false;
let uploadStats = { total: 0, completed: 0, failed: 0 };
let isDrawerMinimized = false;

// --- SVG Icons Map (Lucide-based) ---
const ICONS = {
    folder: `<path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/>`,
    "folder-plus": `<path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/><line x1="12" y1="10" x2="12" y2="16"/><line x1="9" y1="13" x2="15" y2="13"/>`,
    file: `<path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/>`,
    "file-text": `<path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><line x1="10" y1="9" x2="8" y2="9"/>`,
    film: `<rect width="20" height="20" x="2" y="2" rx="2.18" ry="2.18"/><line x1="7" y1="2" x2="7" y2="22"/><line x1="17" y1="2" x2="17" y2="22"/><line x1="2" y1="12" x2="22" y2="12"/><line x1="2" y1="7" x2="7" y2="7"/><line x1="2" y1="17" x2="7" y2="17"/><line x1="17" y1="17" x2="22" y2="17"/><line x1="17" y1="7" x2="22" y2="7"/>`,
    music: `<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>`,
    image: `<rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/>`,
    download: `<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>`,
    upload: `<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>`,
    "share-2": `<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>`,
    "trash-2": `<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" y1="11" x2="10" y2="17"/><line x1="14" y1="11" x2="14" y2="17"/>`,
    edit: `<path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/>`,
    sun: `<circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/>`,
    moon: `<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>`,
    search: `<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`,
    x: `<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`,
    check: `<polyline points="20 6 9 17 4 12"/>`,
    "alert-circle": `<circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>`,
    info: `<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>`,
    "grid": `<rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/>`,
    "list": `<line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>`,
    "chevron-right": `<polyline points="9 18 15 12 9 6"/>`,
    "chevron-down": `<polyline points="6 9 12 15 18 9"/>`,
    "chevron-up": `<polyline points="18 15 12 9 6 15"/>`,
    "more-vertical": `<circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/>`,
    copy: `<rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>`,
    "qr-code": `<rect width="5" height="5" x="3" y="3" rx="1"/><rect width="5" height="5" x="16" y="3" rx="1"/><rect width="5" height="5" x="3" y="16" rx="1"/><path d="M21 16h-3a2 2 0 0 0-2 2v3"/><path d="M21 21v.01"/><path d="M12 7v3a2 2 0 0 1-2 2H7"/><path d="M3 12h.01"/><path d="M12 3h.01"/><path d="M12 16v.01"/><path d="M16 12h1"/><path d="M21 12v.01"/><path d="M12 21v-1"/>`,
    lock: `<rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>`,
    menu: `<line x1="4" y1="12" x2="20" y2="12"/><line x1="4" y1="6" x2="20" y2="6"/><line x1="4" y1="18" x2="20" y2="18"/>`,
    zap: `<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/>`,
    shield: `<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>`,
    database: `<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/><path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/>`,
    "rotate-ccw": `<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>`,
    "help-circle": `<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><line x1="12" y1="17" x2="12.01" y2="17"/>`
};

function getIcon(name, extraClasses = "") {
    const path = ICONS[name] || ICONS.file;
    return `<svg class="icon ${extraClasses}" viewBox="0 0 24 24">${path}</svg>`;
}

// --- Initialization ---
document.addEventListener("DOMContentLoaded", () => {
    initTheme();
    setupDropzone();
    setupSearch();
    setupMobileSidebar();
    loadDriveContent();
    updateViewButtons();
    checkSystemUpdate();
    loadTelegramStatus();
});


// --- Theme Management ---
function initTheme() {
    const savedTheme = localStorage.getItem("teledrive_theme");
    const systemDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const theme = savedTheme || (systemDark ? "dark" : "dark"); // Default dark
    setTheme(theme);
}

function toggleTheme() {
    const current = document.documentElement.getAttribute("data-theme") || "dark";
    const next = current === "dark" ? "light" : "dark";
    setTheme(next);
}

function setTheme(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("teledrive_theme", theme);
    const themeBtn = document.getElementById("theme-toggle-btn");
    if (themeBtn) {
        themeBtn.innerHTML = theme === "dark" ? getIcon("sun") : getIcon("moon");
        themeBtn.setAttribute("title", `Switch to ${theme === "dark" ? "Light" : "Dark"} mode`);
    }
}

// --- Toast System ---
function showToast(message, type = "info", duration = 3500) {
    let container = document.getElementById("toast-container");
    if (!container) {
        container = document.createElement("div");
        container.id = "toast-container";
        container.className = "toast-container";
        document.body.appendChild(container);
    }

    const toast = document.createElement("div");
    toast.className = `toast ${type}`;
    const iconName = type === "success" ? "check" : type === "error" ? "alert-circle" : "info";
    toast.innerHTML = `
        <span style="color: var(--${type === "error" ? "danger" : type === "success" ? "success" : "accent"});">
            ${getIcon(iconName)}
        </span>
        <div class="toast-content">${escapeHtml(message)}</div>
        <button class="btn-icon" onclick="this.parentElement.remove()">${getIcon("x", "icon-sm")}</button>
    `;

    container.appendChild(toast);
    setTimeout(() => {
        if (toast.parentElement) toast.remove();
    }, duration);
}

// --- Navigation Tabs ---
function switchTab(tab) {
    currentTab = tab;
    document.querySelectorAll(".nav-link").forEach(el => el.classList.remove("active"));
    const navEl = document.getElementById(`nav-${tab}`);
    if (navEl) navEl.classList.add("active");

    const driveContainer = document.getElementById("drive-content-container");
    const sharesContainer = document.getElementById("shares-content-container");
    const snapshotsContainer = document.getElementById("snapshots-content-container");

    if (driveContainer) driveContainer.style.display = (tab === "drive") ? "block" : "none";
    if (sharesContainer) sharesContainer.style.display = (tab === "shares") ? "block" : "none";
    if (snapshotsContainer) snapshotsContainer.style.display = (tab === "snapshots") ? "block" : "none";

    if (tab === "drive") {
        loadDriveContent();
    } else if (tab === "shares") {
        loadSharesContent();
    } else if (tab === "snapshots") {
        loadSnapshotsContent();
    }
}

// --- Drive Content Loading ---
async function loadDriveContent() {
    try {
        const folderParam = currentFolderId ? `?folder_id=${currentFolderId}` : "";
        const [foldersRes, filesRes] = await Promise.all([
            fetch(`/api/folders${folderParam}`),
            fetch(`/api/files${folderParam}`)
        ]);

        if (!foldersRes.ok || !filesRes.ok) throw new Error("Failed to fetch files/folders");

        cachedFolders = await foldersRes.json() || [];
        cachedFiles = await filesRes.json() || [];

        renderBreadcrumbs();
        renderActiveView();
    } catch (err) {
        showToast("Error loading files: " + err.message, "error");
    }
}

// --- Breadcrumbs ---
function renderBreadcrumbs() {
    const el = document.getElementById("breadcrumbs");
    if (!el) return;

    if (activeSearchQuery) {
        el.innerHTML = `
            <span class="breadcrumb-item" onclick="clearSearch()" ondragover="handleDragOver(event)" ondragleave="handleDragLeave(event)" ondrop="handleDropOnTarget(event, null)">
                ${getIcon("folder", "icon-sm")} Drive
            </span>
            <span class="breadcrumb-separator">${getIcon("chevron-right", "icon-sm")}</span>
            <span class="breadcrumb-item active">Search results for "${escapeHtml(activeSearchQuery)}"</span>
        `;
        return;
    }

    let html = `
        <span class="breadcrumb-item ${currentFolderId === null ? "active" : ""}" onclick="navigateTo(null, 'Drive')"
              ondragover="handleDragOver(event)" ondragleave="handleDragLeave(event)" ondrop="handleDropOnTarget(event, null)">
            ${getIcon("folder", "icon-sm")} Drive
        </span>
    `;

    currentFolderPath.forEach((crumb, idx) => {
        html += `<span class="breadcrumb-separator">${getIcon("chevron-right", "icon-sm")}</span>`;
        if (idx === currentFolderPath.length - 1) {
            html += `<span class="breadcrumb-item active" ondragover="handleDragOver(event)" ondragleave="handleDragLeave(event)" ondrop="handleDropOnTarget(event, '${crumb.id}')">${escapeHtml(crumb.name)}</span>`;
        } else {
            html += `<span class="breadcrumb-item" onclick="navigateTo('${crumb.id}', '${escapeHtml(crumb.name)}')"
                           ondragover="handleDragOver(event)" ondragleave="handleDragLeave(event)" ondrop="handleDropOnTarget(event, '${crumb.id}')">${escapeHtml(crumb.name)}</span>`;
        }
    });

    el.innerHTML = html;
}

function navigateTo(folderId, folderName) {
    deselectAll();
    if (folderId === null) {
        currentFolderId = null;
        currentFolderPath = [];
    } else {
        const idx = currentFolderPath.findIndex(f => f.id === folderId);
        if (idx >= 0) {
            currentFolderPath = currentFolderPath.slice(0, idx + 1);
        } else {
            currentFolderPath.push({ id: folderId, name: folderName });
        }
        currentFolderId = folderId;
    }
    clearSearch(false);
    loadDriveContent();
}

// --- View Switching (Grid vs List/Table) ---
function setViewMode(mode) {
    currentView = mode;
    localStorage.setItem("teledrive_view", mode);
    updateViewButtons();
    renderActiveView();
}

function updateViewButtons() {
    const gridBtn = document.getElementById("btn-view-grid");
    const listBtn = document.getElementById("btn-view-list");
    if (gridBtn && listBtn) {
        gridBtn.classList.toggle("active", currentView === "grid");
        listBtn.classList.toggle("active", currentView === "list");
    }
}

function renderActiveView() {
    const folders = sortItems([...cachedFolders], currentSort.field, currentSort.order);
    const files = sortItems([...cachedFiles], currentSort.field, currentSort.order);

    if (currentView === "grid") {
        document.getElementById("grid-view-container").style.display = "block";
        document.getElementById("list-view-container").style.display = "none";
        renderFoldersGrid(folders);
        renderFilesGrid(files);
    } else {
        document.getElementById("grid-view-container").style.display = "none";
        document.getElementById("list-view-container").style.display = "block";
        renderTableList(folders, files);
    }
}

// --- Sorting ---
function setSort(field) {
    if (currentSort.field === field) {
        currentSort.order = currentSort.order === "asc" ? "desc" : "asc";
    } else {
        currentSort.field = field;
        currentSort.order = "asc";
    }
    renderActiveView();
}

function sortItems(items, field, order) {
    return items.sort((a, b) => {
        let valA = a[field];
        let valB = b[field];

        if (field === "name") {
            valA = (a.name || "").toLowerCase();
            valB = (b.name || "").toLowerCase();
            return order === "asc" ? valA.localeCompare(valB) : valB.localeCompare(valA);
        }
        if (field === "size") {
            valA = a.size || 0;
            valB = b.size || 0;
            return order === "asc" ? valA - valB : valB - valA;
        }
        if (field === "date") {
            valA = new Date(a.updated_at || a.created_at || 0).getTime();
            valB = new Date(b.updated_at || b.created_at || 0).getTime();
            return order === "asc" ? valA - valB : valB - valA;
        }
        return 0;
    });
}

// --- Grid Rendering ---
function renderFoldersGrid(folders) {
    const container = document.getElementById("folders-grid");
    const section = document.getElementById("folders-section");
    if (!container) return;

    if (folders.length === 0) {
        section.style.display = "none";
        return;
    }
    section.style.display = "block";

    container.innerHTML = folders.map(f => {
        const checked = isSelected("folder", f.id);
        return `
        <div class="card ${checked ? "selected" : ""}" data-id="${f.id}" data-type="folder"
             draggable="true"
             ondragstart="handleDragStart(event, '${f.id}', 'folder')"
             ondragend="handleDragEnd(event)"
             ondragover="handleDragOver(event)"
             ondragleave="handleDragLeave(event)"
             ondrop="handleDropOnTarget(event, '${f.id}')"
             onclick="navigateTo('${f.id}', '${escapeHtml(f.name)}')">
            <div class="item-checkbox ${checked ? "checked" : ""}" onclick="toggleSelectItem('folder', '${f.id}', event)"></div>
            <div class="card-top">
                <div class="card-icon-wrap folder">${getIcon("folder")}</div>
                <div class="card-actions" onclick="event.stopPropagation()">
                    <button class="btn-icon" title="Share Virtual Folder" onclick="openShareModal('${f.id}', '${escapeHtml(f.name)}', 'folder')">${getIcon("share-2", "icon-sm")}</button>
                    <button class="btn-icon" title="Move to..." onclick="openMoveModal('${f.id}', 'folder', '${escapeHtml(f.name)}')"><svg class="icon icon-sm" viewBox="0 0 24 24"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg></button>
                    <button class="btn-icon" title="Rename" onclick="promptRenameFolder('${f.id}', '${escapeHtml(f.name)}')">${getIcon("edit", "icon-sm")}</button>
                    <button class="btn-icon" title="Delete" onclick="confirmDeleteFolder('${f.id}', '${escapeHtml(f.name)}')">${getIcon("trash-2", "icon-sm")}</button>
                </div>
            </div>
            <div>
                <div class="card-title" title="${escapeHtml(f.name)}">${escapeHtml(f.name)}</div>
                <div class="card-meta">Virtual Folder</div>
            </div>
        </div>
    `;
    }).join("");
}

function renderFilesGrid(files) {
    const container = document.getElementById("files-grid");
    const section = document.getElementById("files-section");
    const emptyState = document.getElementById("empty-drive-state");
    if (!container) return;

    if (files.length === 0 && cachedFolders.length === 0) {
        section.style.display = "none";
        if (emptyState) emptyState.style.display = "flex";
        return;
    }

    if (emptyState) emptyState.style.display = "none";

    if (files.length === 0) {
        section.style.display = "none";
        return;
    }
    section.style.display = "block";

    container.innerHTML = files.map(f => {
        const fileType = getFileCategory(f.mime_type, f.name);
        const checked = isSelected("file", f.id);
        return `
            <div class="card ${checked ? "selected" : ""}" data-id="${f.id}" data-type="file"
                 draggable="true"
                 ondragstart="handleDragStart(event, '${f.id}', 'file')"
                 ondragend="handleDragEnd(event)"
                 onclick="openPreview('${f.id}', '${escapeHtml(f.name)}', '${f.mime_type}', ${f.size})">
                <div class="item-checkbox ${checked ? "checked" : ""}" onclick="toggleSelectItem('file', '${f.id}', event)"></div>
                <div class="card-top">
                    <div class="card-icon-wrap ${fileType.category}">
                        ${getIcon(fileType.icon)}
                    </div>
                    <div class="card-actions" onclick="event.stopPropagation()">
                        <button class="btn-icon" title="Share Link" onclick="openShareModal('${f.id}', '${escapeHtml(f.name)}', 'file')">${getIcon("share-2", "icon-sm")}</button>
                        <button class="btn-icon" title="Move to..." onclick="openMoveModal('${f.id}', 'file', '${escapeHtml(f.name)}')"><svg class="icon icon-sm" viewBox="0 0 24 24"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg></button>
                        <button class="btn-icon" title="Download" onclick="downloadFile('${f.id}', '${escapeHtml(f.name)}')">${getIcon("download", "icon-sm")}</button>
                        <button class="btn-icon" title="Rename" onclick="promptRenameFile('${f.id}', '${escapeHtml(f.name)}')">${getIcon("edit", "icon-sm")}</button>
                        <button class="btn-icon" title="Delete" onclick="confirmDeleteFile('${f.id}', '${escapeHtml(f.name)}')">${getIcon("trash-2", "icon-sm")}</button>
                    </div>
                </div>
                <div>
                    <div class="card-title" title="${escapeHtml(f.name)}">${escapeHtml(f.name)}</div>
                    <div class="card-meta">${formatSize(f.size)} &bull; ${formatDate(f.updated_at || f.created_at)}</div>
                </div>
            </div>
        `;
    }).join("");
}

// --- Table / List View Rendering ---
function renderTableList(folders, files) {
    const tbody = document.getElementById("table-body");
    const emptyState = document.getElementById("empty-drive-state");
    if (!tbody) return;

    if (folders.length === 0 && files.length === 0) {
        if (emptyState) emptyState.style.display = "flex";
        tbody.innerHTML = "";
        return;
    }
    if (emptyState) emptyState.style.display = "none";

    let rowsHtml = "";

    // Folders first
    folders.forEach(f => {
        const checked = isSelected("folder", f.id);
        rowsHtml += `
            <tr class="${checked ? "selected" : ""}" data-id="${f.id}" data-type="folder"
                draggable="true"
                ondragstart="handleDragStart(event, '${f.id}', 'folder')"
                ondragend="handleDragEnd(event)"
                ondragover="handleDragOver(event)"
                ondragleave="handleDragLeave(event)"
                ondrop="handleDropOnTarget(event, '${f.id}')"
                onclick="navigateTo('${f.id}', '${escapeHtml(f.name)}')">
                <td style="text-align: center;" onclick="event.stopPropagation()">
                    <div class="item-checkbox ${checked ? "checked" : ""}" style="position: static; opacity: 1;" onclick="toggleSelectItem('folder', '${f.id}', event)"></div>
                </td>
                <td>
                    <div class="table-name-cell">
                        <span style="color: #f59e0b;">${getIcon("folder")}</span>
                        <span>${escapeHtml(f.name)}</span>
                    </div>
                </td>
                <td style="color: var(--text-muted);">&mdash;</td>
                <td style="color: var(--text-muted);">${formatDate(f.updated_at || f.created_at)}</td>
                <td onclick="event.stopPropagation()">
                    <div style="display: flex; gap: 4px; justify-content: flex-end;">
                        <button class="btn-icon" title="Share Virtual Folder" onclick="openShareModal('${f.id}', '${escapeHtml(f.name)}', 'folder')">${getIcon("share-2", "icon-sm")}</button>
                        <button class="btn-icon" title="Move to..." onclick="openMoveModal('${f.id}', 'folder', '${escapeHtml(f.name)}')"><svg class="icon icon-sm" viewBox="0 0 24 24"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg></button>
                        <button class="btn-icon" title="Rename" onclick="promptRenameFolder('${f.id}', '${escapeHtml(f.name)}')">${getIcon("edit", "icon-sm")}</button>
                        <button class="btn-icon" title="Delete" onclick="confirmDeleteFolder('${f.id}', '${escapeHtml(f.name)}')">${getIcon("trash-2", "icon-sm")}</button>
                    </div>
                </td>
            </tr>
        `;
    });

    // Files
    files.forEach(f => {
        const fileType = getFileCategory(f.mime_type, f.name);
        const checked = isSelected("file", f.id);
        rowsHtml += `
            <tr class="${checked ? "selected" : ""}" data-id="${f.id}" data-type="file"
                draggable="true"
                ondragstart="handleDragStart(event, '${f.id}', 'file')"
                ondragend="handleDragEnd(event)"
                onclick="openPreview('${f.id}', '${escapeHtml(f.name)}', '${f.mime_type}', ${f.size})">
                <td style="text-align: center;" onclick="event.stopPropagation()">
                    <div class="item-checkbox ${checked ? "checked" : ""}" style="position: static; opacity: 1;" onclick="toggleSelectItem('file', '${f.id}', event)"></div>
                </td>
                <td>
                    <div class="table-name-cell">
                        <span class="card-icon-wrap ${fileType.category}" style="width: 28px; height: 28px;">
                            ${getIcon(fileType.icon, "icon-sm")}
                        </span>
                        <span title="${escapeHtml(f.name)}">${escapeHtml(f.name)}</span>
                    </div>
                </td>
                <td>${formatSize(f.size)}</td>
                <td style="color: var(--text-muted);">${formatDate(f.updated_at || f.created_at)}</td>
                <td onclick="event.stopPropagation()">
                    <div style="display: flex; gap: 4px; justify-content: flex-end;">
                        <button class="btn-icon" title="Share Link" onclick="openShareModal('${f.id}', '${escapeHtml(f.name)}', 'file')">${getIcon("share-2", "icon-sm")}</button>
                        <button class="btn-icon" title="Move to..." onclick="openMoveModal('${f.id}', 'file', '${escapeHtml(f.name)}')"><svg class="icon icon-sm" viewBox="0 0 24 24"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg></button>
                        <button class="btn-icon" title="Download" onclick="downloadFile('${f.id}', '${escapeHtml(f.name)}')">${getIcon("download", "icon-sm")}</button>
                        <button class="btn-icon" title="Rename" onclick="promptRenameFile('${f.id}', '${escapeHtml(f.name)}')">${getIcon("edit", "icon-sm")}</button>
                        <button class="btn-icon" title="Delete" onclick="confirmDeleteFile('${f.id}', '${escapeHtml(f.name)}')">${getIcon("trash-2", "icon-sm")}</button>
                    </div>
                </td>
            </tr>
        `;
    });

    tbody.innerHTML = rowsHtml;
}

// --- File Type Resolver ---
function getFileCategory(mime, filename = "") {
    mime = (mime || "").toLowerCase();
    const ext = filename.split(".").pop().toLowerCase();

    if (mime.startsWith("video/") || ["mp4", "mkv", "avi", "mov", "webm"].includes(ext)) {
        return { category: "video", icon: "film" };
    }
    if (mime.startsWith("audio/") || ["mp3", "flac", "wav", "ogg", "m4a"].includes(ext)) {
        return { category: "audio", icon: "music" };
    }
    if (mime.startsWith("image/") || ["jpg", "jpeg", "png", "gif", "webp", "svg"].includes(ext)) {
        return { category: "image", icon: "image" };
    }
    if (mime === "application/pdf" || ext === "pdf") {
        return { category: "pdf", icon: "file-text" };
    }
    if (mime.startsWith("text/") || ["txt", "md", "json", "go", "py", "js", "html", "css", "yaml", "yml", "sql", "sh"].includes(ext)) {
        return { category: "code", icon: "file-text" };
    }
    return { category: "file", icon: "file" };
}

// --- Search Handling ---
function setupSearch() {
    const input = document.getElementById("search-box");
    const clearBtn = document.getElementById("search-clear-btn");
    if (!input) return;

    input.addEventListener("input", (e) => {
        const q = e.target.value.trim();
        if (clearBtn) clearBtn.style.display = q ? "block" : "none";

        clearTimeout(searchDebounceTimer);
        searchDebounceTimer = setTimeout(() => {
            performSearch(q);
        }, 250);
    });
}

async function performSearch(query) {
    activeSearchQuery = query;
    const banner = document.getElementById("search-banner");
    const bannerText = document.getElementById("search-banner-text");

    if (!query) {
        if (banner) banner.style.display = "none";
        loadDriveContent();
        return;
    }

    try {
        const res = await fetch(`/api/files?search=${encodeURIComponent(query)}`);
        const files = await res.json() || [];
        cachedFolders = []; // Search results focus on matched virtual files
        cachedFiles = files;

        if (banner && bannerText) {
            banner.style.display = "flex";
            bannerText.innerText = `Found ${files.length} file${files.length === 1 ? "" : "s"} matching "${query}"`;
        }

        renderBreadcrumbs();
        renderActiveView();
    } catch (err) {
        showToast("Search failed: " + err.message, "error");
    }
}

function clearSearch(reload = true) {
    const input = document.getElementById("search-box");
    const clearBtn = document.getElementById("search-clear-btn");
    const banner = document.getElementById("search-banner");

    if (input) input.value = "";
    if (clearBtn) clearBtn.style.display = "none";
    if (banner) banner.style.display = "none";
    activeSearchQuery = "";

    if (reload) {
        loadDriveContent();
    }
}

// --- Shared Links View & API ---
async function loadSharesContent() {
    try {
        const res = await fetch("/api/shares");
        if (!res.ok) throw new Error("Failed to load share links");
        const shares = await res.json() || [];

        const tbody = document.getElementById("shares-table-body");
        const emptyState = document.getElementById("empty-shares-state");

        if (shares.length === 0) {
            if (emptyState) emptyState.style.display = "flex";
            if (tbody) tbody.innerHTML = "";
            return;
        }

        if (emptyState) emptyState.style.display = "none";
        if (!tbody) return;

        tbody.innerHTML = shares.map(s => {
            const shareUrl = `${window.location.origin}/s/${s.token}`;
            const isExpired = s.expires_at && new Date(s.expires_at) < new Date();
            const isFolder = s.type === "folder";
            const targetName = s.target_name || s.file_name;
            return `
                <tr>
                    <td>
                        <div class="table-name-cell">
                            <span style="color: ${isFolder ? "#f59e0b" : "var(--primary)"};">${getIcon(isFolder ? "folder" : "share-2")}</span>
                            <span style="font-weight: 600;">${escapeHtml(targetName)}</span>
                            ${isFolder ? '<span style="font-size: 0.7rem; background: rgba(245, 158, 11, 0.15); color: #f59e0b; padding: 2px 6px; border-radius: 4px; font-weight: 600;">Folder</span>' : ''}
                        </div>
                    </td>
                    <td>${isFolder ? "&mdash;" : formatSize(s.file_size)}</td>
                    <td>
                        ${s.has_password 
                            ? `<span style="color: var(--warning); display: flex; align-items: center; gap: 4px;">${getIcon("lock", "icon-sm")} Protected</span>` 
                            : `<span style="color: var(--text-muted);">None</span>`}
                    </td>
                    <td>
                        ${s.expires_at 
                            ? `<span style="color: ${isExpired ? "var(--danger)" : "var(--text-secondary)"};">${formatDate(s.expires_at)} ${isExpired ? "(Expired)" : ""}</span>` 
                            : `<span style="color: var(--text-muted);">Never</span>`}
                    </td>
                    <td>${s.download_count}</td>
                    <td>
                        <div style="display: flex; gap: 6px;">
                            <button class="btn btn-secondary" style="padding: 4px 10px; font-size: 0.75rem;" onclick="copyToClipboard('${shareUrl}')" title="Copy Link">
                                ${getIcon("copy", "icon-sm")} Copy
                            </button>
                            <button class="btn btn-secondary" style="padding: 4px 10px; font-size: 0.75rem;" onclick="openQrModal('${shareUrl}', '${escapeHtml(s.file_name)}')" title="Show QR Code">
                                ${getIcon("qr-code", "icon-sm")} QR
                            </button>
                            <button class="btn-icon btn-danger" style="padding: 4px 6px;" onclick="confirmRevokeShare('${s.id}', '${escapeHtml(s.file_name)}')" title="Revoke Share">
                                ${getIcon("trash-2", "icon-sm")}
                            </button>
                        </div>
                    </td>
                </tr>
            `;
        }).join("");
    } catch (err) {
        showToast("Failed to fetch shares: " + err.message, "error");
    }
}

function confirmRevokeShare(id, fileName) {
    showConfirmDialog({
        title: "Revoke Share Link",
        message: `Are you sure you want to revoke the share link for "${fileName}"? Anyone with this URL will immediately lose access.`,
        confirmText: "Revoke Link",
        danger: true,
        onConfirm: async () => {
            try {
                const res = await fetch(`/api/shares/${id}`, { method: "DELETE" });
                if (!res.ok) throw new Error("Revoke failed");
                showToast("Share link revoked successfully", "success");
                loadSharesContent();
            } catch (err) {
                showToast(err.message, "error");
            }
        }
    });
}

// --- Database Snapshots & Point-in-Time Restore ---
async function loadSnapshotsContent() {
    try {
        const res = await fetch("/api/snapshots");
        if (!res.ok) throw new Error("Failed to load snapshots");
        const snapshots = await res.json() || [];

        const tbody = document.getElementById("snapshots-table-body");
        const emptyState = document.getElementById("empty-snapshots-state");

        if (snapshots.length === 0) {
            if (emptyState) emptyState.style.display = "flex";
            if (tbody) tbody.innerHTML = "";
            return;
        }

        if (emptyState) emptyState.style.display = "none";
        if (!tbody) return;

        tbody.innerHTML = snapshots.map(s => {
            const statusBadge = s.is_pinned
                ? `<span style="color: var(--warning); font-size: 0.8rem; font-weight: 600; display: inline-flex; align-items: center; gap: 4px;">Pinned</span>`
                : `<span style="color: var(--text-muted); font-size: 0.8rem;">Archived</span>`;

            return `
                <tr>
                    <td>
                        <div class="table-name-cell">
                            <span style="color: var(--primary);">${getIcon("database")}</span>
                            <span style="font-weight: 600; font-family: monospace; font-size: 0.85rem;">${escapeHtml(s.file_name)}</span>
                        </div>
                    </td>
                    <td>${formatSize(s.size)}</td>
                    <td><span style="font-family: monospace; font-size: 0.85rem; color: var(--text-secondary);">#${s.message_id}</span></td>
                    <td>${formatDateTime(s.created_at)}</td>
                    <td>${statusBadge}</td>
                    <td>
                        <div style="display: flex; gap: 6px; justify-content: flex-end;">
                            <button class="btn btn-secondary" style="padding: 4px 10px; font-size: 0.75rem; color: var(--warning);" onclick="confirmRestoreSnapshot(${s.message_id}, '${escapeHtml(s.file_name)}')" title="Restore this snapshot">
                                ${getIcon("rotate-ccw", "icon-sm")} Restore
                            </button>
                            <button class="btn btn-secondary" style="padding: 4px 10px; font-size: 0.75rem;" onclick="downloadSnapshot(${s.message_id})" title="Download .db.gz">
                                ${getIcon("download", "icon-sm")} Download
                            </button>
                            <button class="btn-icon btn-danger" style="padding: 4px 6px;" onclick="confirmDeleteSnapshot(${s.message_id}, '${escapeHtml(s.file_name)}')" title="Delete snapshot">
                                ${getIcon("trash-2", "icon-sm")}
                            </button>
                        </div>
                    </td>
                </tr>
            `;
        }).join("");
    } catch (err) {
        showToast("Failed to fetch snapshots: " + err.message, "error");
    }
}

async function createSnapshotNow() {
    const btn = document.getElementById("btn-create-snapshot");
    const textSpan = document.getElementById("create-snapshot-text");
    if (btn) btn.disabled = true;
    if (textSpan) textSpan.innerText = "Creating snapshot...";

    try {
        const res = await fetch("/api/snapshots", { method: "POST" });
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || "Failed to create snapshot");
        }
        showToast("Snapshot created and uploaded to Telegram Storage Channel!", "success");
        await loadSnapshotsContent();
    } catch (err) {
        showToast("Snapshot error: " + err.message, "error");
    } finally {
        if (btn) btn.disabled = false;
        if (textSpan) textSpan.innerText = "Create Snapshot Now";
    }
}

function confirmRestoreSnapshot(id, fileName) {
    showConfirmDialog({
        title: "Point-in-Time Restore",
        message: `Are you sure you want to restore snapshot "${fileName}"? This will overwrite your current database. The page will reload once restore finishes.`,
        confirmText: "Restore Snapshot",
        danger: true,
        onConfirm: async () => {
            const overlay = document.getElementById("restore-overlay-modal");
            const overlayTitle = document.getElementById("restore-overlay-title");
            const overlayMsg = document.getElementById("restore-overlay-msg");
            if (overlay) overlay.style.display = "flex";
            if (overlayTitle) overlayTitle.innerText = "Restoring Database...";
            if (overlayMsg) overlayMsg.innerText = "Draining connections and applying SQLite snapshot. Please wait...";

            try {
                const res = await fetch(`/api/snapshots/${id}/restore`, { method: "POST" });
                if (!res.ok) {
                    const errText = await res.text();
                    throw new Error(errText || "Failed to restore snapshot");
                }
                if (overlayTitle) overlayTitle.innerText = "Restore Complete!";
                if (overlayMsg) overlayMsg.innerText = "Database successfully restored. Reloading page...";
                setTimeout(() => {
                    window.location.reload();
                }, 2000);
            } catch (err) {
                if (overlay) overlay.style.display = "none";
                showToast("Restore failed: " + err.message, "error", 6000);
            }
        }
    });
}

function downloadSnapshot(id) {
    window.location.href = `/api/snapshots/${id}/download`;
}

function confirmDeleteSnapshot(id, fileName) {
    showConfirmDialog({
        title: "Delete Snapshot",
        message: `Are you sure you want to delete snapshot "${fileName}" (Telegram Msg #${id}) from the Storage Channel?`,
        confirmText: "Delete Snapshot",
        danger: true,
        onConfirm: async () => {
            try {
                const res = await fetch(`/api/snapshots/${id}`, { method: "DELETE" });
                if (!res.ok) {
                    const errText = await res.text();
                    throw new Error(errText || "Failed to delete snapshot");
                }
                showToast("Snapshot deleted from Telegram Storage Channel", "success");
                loadSnapshotsContent();
            } catch (err) {
                showToast("Delete failed: " + err.message, "error");
            }
        }
    });
}

function handleSnapshotFileSelect(event) {
    const file = event.target.files && event.target.files[0];
    if (!file) return;

    // Reset input value so same file can be re-selected if needed
    event.target.value = "";

    showConfirmDialog({
        title: "Upload & Restore Snapshot",
        message: `Are you sure you want to restore from "${file.name}"? Current database contents will be replaced with this backup file.`,
        confirmText: "Upload & Overwrite",
        danger: true,
        onConfirm: async () => {
            const overlay = document.getElementById("restore-overlay-modal");
            const overlayTitle = document.getElementById("restore-overlay-title");
            const overlayMsg = document.getElementById("restore-overlay-msg");
            if (overlay) overlay.style.display = "flex";
            if (overlayTitle) overlayTitle.innerText = "Uploading & Restoring...";
            if (overlayMsg) overlayMsg.innerText = "Uploading local snapshot and applying to database. Please wait...";

            const formData = new FormData();
            formData.append("snapshot", file);

            try {
                const res = await fetch("/api/snapshots/upload-restore", {
                    method: "POST",
                    body: formData
                });
                if (!res.ok) {
                    const errText = await res.text();
                    throw new Error(errText || "Upload restore failed");
                }
                if (overlayTitle) overlayTitle.innerText = "Restore Complete!";
                if (overlayMsg) overlayMsg.innerText = "Local backup successfully restored. Reloading page...";
                setTimeout(() => {
                    window.location.reload();
                }, 2000);
            } catch (err) {
                if (overlay) overlay.style.display = "none";
                showToast("Upload restore failed: " + err.message, "error", 6000);
            }
        }
    });
}

// --- Upload Management (Sequential Safe Mode & Recursive Folder Upload) ---
let batchConflictPolicy = null;
let conflictResolver = null;

function setupDropzone() {
    const dropzone = document.getElementById("dropzone");
    const fileInput = document.getElementById("file-input");
    const folderInput = document.getElementById("folder-input");

    if (!dropzone) return;

    ["dragenter", "dragover"].forEach(evt => {
        dropzone.addEventListener(evt, e => {
            e.preventDefault();
            dropzone.classList.add("dragover");
        });
    });

    ["dragleave", "drop"].forEach(evt => {
        dropzone.addEventListener(evt, e => {
            e.preventDefault();
            dropzone.classList.remove("dragover");
        });
    });

    dropzone.addEventListener("drop", async e => {
        e.preventDefault();
        dropzone.classList.remove("dragover");
        const scanned = await scanDataTransfer(e.dataTransfer);
        if (scanned.length > 0) {
            handleQueueItems(scanned);
        }
    });

    if (fileInput) {
        fileInput.addEventListener("change", e => {
            if (e.target.files.length > 0) {
                const items = Array.from(e.target.files).map(f => ({ file: f, path: f.name }));
                handleQueueItems(items);
                fileInput.value = "";
            }
        });
    }

    if (folderInput) {
        folderInput.addEventListener("change", e => {
            if (e.target.files.length > 0) {
                const items = Array.from(e.target.files).map(f => ({
                    file: f,
                    path: f.webkitRelativePath || f.name
                }));
                handleQueueItems(items);
                folderInput.value = "";
            }
        });
    }
}

async function scanDataTransfer(dataTransfer) {
    const items = dataTransfer.items;
    if (!items || items.length === 0) {
        return Array.from(dataTransfer.files || []).map(f => ({ file: f, path: f.name }));
    }

    const fileEntries = [];

    async function traverseEntry(entry, currentPath = "") {
        if (!entry) return;
        if (entry.isFile) {
            return new Promise((resolve) => {
                entry.file((file) => {
                    fileEntries.push({
                        file: file,
                        path: (currentPath ? currentPath + "/" : "") + file.name
                    });
                    resolve();
                }, () => resolve());
            });
        } else if (entry.isDirectory) {
            const dirReader = entry.createReader();
            const readEntries = () => new Promise((resolve) => {
                dirReader.readEntries(resolve, () => resolve([]));
            });

            const nextPath = currentPath ? currentPath + "/" + entry.name : entry.name;
            let entries = await readEntries();
            while (entries && entries.length > 0) {
                for (const child of entries) {
                    await traverseEntry(child, nextPath);
                }
                entries = await readEntries();
            }
        }
    }

    const promises = [];
    for (let i = 0; i < items.length; i++) {
        const entry = items[i].webkitGetAsEntry ? items[i].webkitGetAsEntry() : null;
        if (entry) {
            promises.push(traverseEntry(entry));
        } else if (items[i].kind === "file") {
            const f = items[i].getAsFile();
            if (f) fileEntries.push({ file: f, path: f.name });
        }
    }
    await Promise.all(promises);
    return fileEntries;
}

function askConflictResolution(fileName) {
    if (batchConflictPolicy) {
        return Promise.resolve(batchConflictPolicy);
    }
    return new Promise((resolve) => {
        const modal = document.getElementById("conflict-modal");
        const msg = document.getElementById("conflict-message");
        const applyCheck = document.getElementById("conflict-apply-all");
        if (applyCheck) applyCheck.checked = false;
        if (msg) msg.innerText = `An item named "${fileName}" already exists in this folder. How would you like to proceed?`;
        if (modal) modal.style.display = "flex";

        conflictResolver = (choice) => {
            if (modal) modal.style.display = "none";
            if (applyCheck && applyCheck.checked) {
                batchConflictPolicy = choice;
            }
            resolve(choice);
        };
    });
}

function resolveConflict(choice) {
    if (conflictResolver) {
        const fn = conflictResolver;
        conflictResolver = null;
        fn(choice);
    }
}

function generateUniqueFileName(name, existingNames) {
    const extIdx = name.lastIndexOf(".");
    const base = extIdx > 0 ? name.substring(0, extIdx) : name;
    const ext = extIdx > 0 ? name.substring(extIdx) : "";

    let counter = 1;
    let candidate = `${base} (${counter})${ext}`;
    while (existingNames.includes(candidate)) {
        counter++;
        candidate = `${base} (${counter})${ext}`;
    }
    return candidate;
}

async function ensureFolderPath(dirSegments, rootParentId, cache) {
    let currentParent = rootParentId;
    for (const dirName of dirSegments) {
        const cacheKey = (currentParent || "root") + "::" + dirName;
        if (cache.has(cacheKey)) {
            currentParent = cache.get(cacheKey);
            continue;
        }

        const url = currentParent ? `/api/folders?parent_id=${encodeURIComponent(currentParent)}` : `/api/folders`;
        const res = await fetch(url);
        let folderId = null;
        if (res.ok) {
            const list = await res.json();
            const found = (list || []).find(f => f.name === dirName);
            if (found) folderId = found.id;
        }

        if (!folderId) {
            const createRes = await fetch("/api/folders", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ name: dirName, parent_id: currentParent })
            });
            if (!createRes.ok) {
                throw new Error(`Could not create virtual folder "${dirName}"`);
            }
            const created = await createRes.json();
            folderId = created.id;
        }

        cache.set(cacheKey, folderId);
        currentParent = folderId;
    }
    return currentParent;
}

async function handleQueueItems(items) {
    if (!items || items.length === 0) return;
    batchConflictPolicy = null;

    const drawer = document.getElementById("upload-drawer");
    if (drawer) drawer.style.display = "block";

    const folderCache = new Map();
    const baseFolderId = currentFolderId;
    const filesInFolderCache = new Map();

    async function getFilesInFolder(fId) {
        const key = fId || "root";
        if (filesInFolderCache.has(key)) {
            return filesInFolderCache.get(key);
        }
        try {
            const url = fId ? `/api/files?folder_id=${encodeURIComponent(fId)}` : `/api/files`;
            const res = await fetch(url);
            if (res.ok) {
                const files = await res.json();
                filesInFolderCache.set(key, files || []);
                return files || [];
            }
        } catch (_) {}
        return [];
    }

    for (const item of items) {
        const fullPath = item.path.replace(/^\/+/, "");
        const pathSegments = fullPath.split("/").filter(Boolean);
        const fileName = pathSegments.pop() || item.file.name;
        const dirSegments = pathSegments;

        let targetFolderId = baseFolderId;
        if (dirSegments.length > 0) {
            try {
                targetFolderId = await ensureFolderPath(dirSegments, baseFolderId, folderCache);
            } catch (err) {
                showToast(`Folder creation failed for ${fullPath}: ${err.message}`, "error");
                continue;
            }
        }

        const existingFiles = await getFilesInFolder(targetFolderId);
        const existingFile = existingFiles.find(f => f.name === fileName);

        let finalFileName = fileName;
        let replaceFileId = null;

        if (existingFile) {
            const action = await askConflictResolution(fileName);
            if (action === "skip") {
                continue;
            } else if (action === "keep_both") {
                finalFileName = generateUniqueFileName(fileName, existingFiles.map(f => f.name));
            } else if (action === "replace") {
                replaceFileId = existingFile.id;
            }
        }

        const queueItem = {
            id: "upl_" + Math.random().toString(36).substring(2, 9),
            file: item.file,
            targetFolderId: targetFolderId,
            fileName: finalFileName,
            replaceFileId: replaceFileId,
            status: "queued",
            progress: 0,
            error: null
        };
        uploadQueue.push(queueItem);
        uploadStats.total++;

        existingFiles.push({ name: finalFileName, id: queueItem.id });
        renderUploadDrawer();
    }

    renderUploadDrawer();
    processNextUpload();
}

// Backward compatibility helper
function handleQueueFiles(fileList) {
    const items = Array.from(fileList).map(f => ({ file: f, path: f.name }));
    handleQueueItems(items);
}

async function processNextUpload() {
    if (isUploading) return;
    const currentItem = uploadQueue.find(item => item.status === "queued");
    if (!currentItem) {
        if (uploadQueue.every(i => i.status === "completed" || i.status === "failed")) {
            setTimeout(() => {
                const drawer = document.getElementById("upload-drawer");
                if (drawer && uploadQueue.every(i => i.status === "completed")) {
                    drawer.style.display = "none";
                    uploadQueue = [];
                }
            }, 6000);
        }
        return;
    }

    isUploading = true;
    currentItem.status = "uploading";
    renderUploadDrawer();

    const file = currentItem.file;
    const uploadName = currentItem.fileName || file.name;
    const uploadFolderId = currentItem.targetFolderId !== undefined ? currentItem.targetFolderId : currentFolderId;

    try {
        // 1. Init upload session
        const initRes = await fetch("/api/upload/init", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                name: uploadName,
                size: file.size,
                mime_type: file.type || "application/octet-stream",
                folder_id: uploadFolderId
            })
        });

        if (!initRes.ok) throw new Error("Upload initialization failed");
        const session = await initRes.json();
        const chunkSize = session.chunk_size || 5 * 1024 * 1024;
        const totalChunks = Math.ceil(file.size / chunkSize);

        // 2. Upload chunk by chunk
        for (let chunkIdx = 0; chunkIdx < totalChunks; chunkIdx++) {
            const start = chunkIdx * chunkSize;
            const end = Math.min(file.size, start + chunkSize);
            const chunkBlob = file.slice(start, end);

            const formData = new FormData();
            formData.append("session_id", session.id);
            formData.append("chunk_index", chunkIdx);
            formData.append("chunk", chunkBlob);

            currentItem.chunkStatus = `Chunk ${chunkIdx + 1}/${totalChunks}`;
            renderUploadDrawer();

            let retries = 3;
            let success = false;
            while (retries > 0 && !success) {
                try {
                    const chunkRes = await fetch("/api/upload/chunk", {
                        method: "POST",
                        body: formData
                    });
                    if (chunkRes.ok) success = true;
                    else retries--;
                } catch (e) {
                    retries--;
                    await new Promise(r => setTimeout(r, 1000));
                }
            }
            if (!success) throw new Error(`Failed to upload chunk ${chunkIdx + 1}`);

            currentItem.progress = Math.round((end / file.size) * 100);
            renderUploadDrawer();
        }

        // 3. Complete and commit to Storage Channel
        currentItem.chunkStatus = "Committing to Storage Channel...";
        renderUploadDrawer();

        const completeRes = await fetch("/api/upload/complete", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ session_id: session.id })
        });
        if (!completeRes.ok) throw new Error("Telegram MTProto document assembly failed");

        // If replacing an existing file, clean up the superseded file
        if (currentItem.replaceFileId) {
            try {
                await fetch(`/api/files/${currentItem.replaceFileId}`, { method: "DELETE" });
            } catch (_) {}
        }

        currentItem.status = "completed";
        currentItem.progress = 100;
        currentItem.chunkStatus = "Uploaded to Telegram Vault";
        uploadStats.completed++;
        showToast(`Uploaded ${uploadName}`, "success");
        loadDriveContent();
    } catch (err) {
        currentItem.status = "failed";
        currentItem.error = err.message;
        uploadStats.failed++;
        showToast(`Upload failed: ${uploadName} (${err.message})`, "error");
    } finally {
        isUploading = false;
        renderUploadDrawer();
        processNextUpload();
    }
}

function renderUploadDrawer() {
    const drawer = document.getElementById("upload-drawer");
    const headerTitle = document.getElementById("upload-drawer-title-text");
    const body = document.getElementById("upload-drawer-items");
    if (!drawer || !body) return;

    if (uploadQueue.length === 0) {
        drawer.style.display = "none";
        return;
    }

    const activeCount = uploadQueue.filter(i => i.status === "uploading" || i.status === "queued").length;
    if (headerTitle) {
        headerTitle.innerText = activeCount > 0 
            ? `Uploading (${uploadQueue.length - activeCount}/${uploadQueue.length})` 
            : `Uploads Finished (${uploadStats.completed} completed)`;
    }

    body.innerHTML = uploadQueue.map(item => `
        <div class="upload-item">
            <div class="upload-item-header">
                <span class="upload-item-name" title="${escapeHtml(item.fileName || item.file.name)}">${escapeHtml(item.fileName || item.file.name)}</span>
                <span class="upload-item-status">
                    ${item.status === "uploading" ? (item.chunkStatus || `${item.progress}%`) : ""}
                    ${item.status === "completed" ? `<span style="color: var(--success);">${getIcon("check", "icon-sm")} Done</span>` : ""}
                    ${item.status === "failed" ? `<span style="color: var(--danger);">${getIcon("alert-circle", "icon-sm")} Failed</span>` : ""}
                    ${item.status === "queued" ? "Queued" : ""}
                </span>
            </div>
            <div class="progress-bar-wrap">
                <div class="progress-bar-fill ${item.status === "completed" ? "success" : item.status === "failed" ? "error" : ""}" style="width: ${item.progress}%;"></div>
            </div>
        </div>
    `).join("");
}

function toggleUploadDrawer() {
    const drawer = document.getElementById("upload-drawer");
    if (!drawer) return;
    isDrawerMinimized = !isDrawerMinimized;
    drawer.classList.toggle("minimized", isDrawerMinimized);
    const minIcon = document.getElementById("drawer-min-icon");
    if (minIcon) minIcon.innerHTML = isDrawerMinimized ? getIcon("chevron-up", "icon-sm") : getIcon("chevron-down", "icon-sm");
}

function closeUploadDrawer() {
    const drawer = document.getElementById("upload-drawer");
    if (drawer) drawer.style.display = "none";
}

// --- Preview Modal (Range Streaming & Expanded Formats) ---
async function openPreview(id, name, mime, size) {
    const modal = document.getElementById("preview-modal");
    const container = document.getElementById("preview-content");
    const title = document.getElementById("preview-title");
    const meta = document.getElementById("preview-meta");
    const downloadBtn = document.getElementById("preview-download-btn");

    if (!modal || !container || !title) return;

    title.innerText = name;
    if (meta) meta.innerText = `${formatSize(size)} • ${mime}`;
    if (downloadBtn) downloadBtn.onclick = () => downloadFile(id, name);

    const streamUrl = `/api/files/${id}/stream`;
    const fileCategory = getFileCategory(mime, name).category;

    if (fileCategory === "video") {
        container.innerHTML = `
            <video controls autoplay style="width: 100%; max-height: 65vh; border-radius: var(--radius-sm); outline: none;">
                <source src="${streamUrl}" type="${mime}">
                Your browser does not support HTML5 video streaming.
            </video>
        `;
    } else if (fileCategory === "audio") {
        container.innerHTML = `
            <div style="padding: 40px; text-align: center; width: 100%;">
                <div style="color: #ec4899; margin-bottom: 20px;">${getIcon("music", "icon-xl")}</div>
                <audio controls autoplay style="width: 100%; max-width: 480px;">
                    <source src="${streamUrl}" type="${mime}">
                </audio>
            </div>
        `;
    } else if (fileCategory === "image") {
        container.innerHTML = `
            <img src="${streamUrl}" style="max-width: 100%; max-height: 65vh; object-fit: contain; border-radius: var(--radius-sm);" alt="${escapeHtml(name)}">
        `;
    } else if (fileCategory === "pdf") {
        container.innerHTML = `
            <iframe src="${streamUrl}" style="width: 100%; height: 65vh; border: none; border-radius: var(--radius-sm);"></iframe>
        `;
    } else if (fileCategory === "code") {
        container.innerHTML = `<div style="color: var(--text-muted); padding: 20px;">Loading preview...</div>`;
        try {
            const res = await fetch(streamUrl);
            const text = await res.text();
            container.innerHTML = `<pre class="code-preview">${escapeHtml(text.slice(0, 100000))}</pre>`;
        } catch (e) {
            container.innerHTML = `<p style="color: var(--danger);">Failed to load document text.</p>`;
        }
    } else {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">${getIcon("file", "icon-xl")}</div>
                <h3>Preview not available</h3>
                <p style="font-size: 0.85rem; color: var(--text-muted);">This file type cannot be previewed directly in browser.</p>
                <button class="btn btn-primary" onclick="downloadFile('${id}', '${escapeHtml(name)}')">
                    ${getIcon("download", "icon-sm")} Download File (${formatSize(size)})
                </button>
            </div>
        `;
    }

    modal.style.display = "flex";
}

function closePreview() {
    const modal = document.getElementById("preview-modal");
    const container = document.getElementById("preview-content");
    if (container) container.innerHTML = "";
    if (modal) modal.style.display = "none";
}

// --- Share Modal ---
let activeShareTarget = null; // { id, name, type: "file" | "folder" }

function openShareModal(id, name, type = "file") {
    activeShareTarget = { id, name, type };
    const modal = document.getElementById("share-modal");
    const nameEl = document.getElementById("share-file-name");
    const resultBox = document.getElementById("share-result-box");
    const pwdInput = document.getElementById("share-password");
    const expirySelect = document.getElementById("share-expiry");
    const customDateGroup = document.getElementById("share-custom-date-group");
    const customDateInput = document.getElementById("share-custom-date");

    if (nameEl) {
        nameEl.innerHTML = `${type === "folder" ? "📁 " : "📄 "}<strong>${escapeHtml(name)}</strong> (${type === "folder" ? "Virtual Folder" : "File"})`;
    }
    if (resultBox) resultBox.style.display = "none";
    if (pwdInput) pwdInput.value = "";
    if (expirySelect) expirySelect.value = "0"; // Default: Never expires
    if (customDateGroup) customDateGroup.style.display = "none";
    if (customDateInput) customDateInput.value = "";
    if (modal) modal.style.display = "flex";
}

function handleShareExpiryChange() {
    const val = document.getElementById("share-expiry").value;
    const group = document.getElementById("share-custom-date-group");
    if (group) group.style.display = (val === "custom") ? "block" : "none";
}

function closeShareModal() {
    const modal = document.getElementById("share-modal");
    if (modal) modal.style.display = "none";
    activeShareTarget = null;
}

async function createShareLinkSubmit() {
    if (!activeShareTarget) return;
    const password = document.getElementById("share-password").value.trim();
    const expiryVal = document.getElementById("share-expiry").value;
    let expiryDays = null;
    let expiresAt = null;

    if (expiryVal === "custom") {
        const customDate = document.getElementById("share-custom-date").value;
        if (!customDate) {
            showToast("Please select a custom expiration date", "error");
            return;
        }
        expiresAt = customDate;
    } else {
        const days = parseInt(expiryVal);
        if (days > 0) expiryDays = days;
    }

    try {
        const payload = {
            password: password || null,
            expiry_days: expiryDays,
            expires_at: expiresAt
        };
        if (activeShareTarget.type === "folder") {
            payload.folder_id = activeShareTarget.id;
        } else {
            payload.file_id = activeShareTarget.id;
        }

        const res = await fetch("/api/share", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload)
        });

        if (!res.ok) throw new Error("Failed to create share link");
        const data = await res.json();
        const shareUrl = `${window.location.origin}/s/${data.token}`;

        document.getElementById("share-link-input").value = shareUrl;
        document.getElementById("share-result-box").style.display = "block";
        showToast("Share link generated!", "success");
    } catch (err) {
        showToast(err.message, "error");
    }
}

function copyShareLinkInput() {
    const input = document.getElementById("share-link-input");
    copyToClipboard(input.value);
}

// --- QR Code Modal ---
function openQrModal(url, title = "Share Link") {
    const modal = document.getElementById("qr-modal");
    const titleEl = document.getElementById("qr-modal-title");
    const container = document.getElementById("qr-container");

    if (!modal || !container) return;
    if (titleEl) titleEl.innerText = title;

    // Lightweight QR SVG Generator via QuickChart SVG API or local fallback
    const qrApiUrl = `https://api.qrserver.com/v1/create-qr-code/?size=220x220&data=${encodeURIComponent(url)}`;
    container.innerHTML = `
        <div style="background: #fff; padding: 16px; border-radius: var(--radius-md); display: inline-block;">
            <img src="${qrApiUrl}" width="220" height="220" alt="QR Code" style="display: block;">
        </div>
        <p style="font-size: 0.8rem; color: var(--text-muted); margin-top: 12px; word-break: break-all;">
            ${escapeHtml(url)}
        </p>
    `;
    modal.style.display = "flex";
}

function closeQrModal() {
    const modal = document.getElementById("qr-modal");
    if (modal) modal.style.display = "none";
}

// --- Help & Guide Modal ---
function openHelpModal() {
    const modal = document.getElementById("help-modal");
    if (modal) modal.style.display = "flex";
}

function closeHelpModal() {
    const modal = document.getElementById("help-modal");
    if (modal) modal.style.display = "none";
}

// --- Custom Prompt & Confirm Dialogs (Replacing prompt/confirm) ---
function showPromptDialog({ title, label, defaultValue = "", placeholder = "", confirmText = "Save", onConfirm }) {
    const modal = document.getElementById("prompt-modal");
    const titleEl = document.getElementById("prompt-title");
    const labelEl = document.getElementById("prompt-label");
    const inputEl = document.getElementById("prompt-input");
    const submitBtn = document.getElementById("prompt-submit-btn");

    if (!modal || !inputEl) return;

    titleEl.innerText = title;
    labelEl.innerText = label;
    inputEl.value = defaultValue;
    inputEl.placeholder = placeholder;
    submitBtn.innerText = confirmText;

    modal.style.display = "flex";
    setTimeout(() => inputEl.focus(), 50);

    const handleSubmit = () => {
        const val = inputEl.value.trim();
        if (val) {
            modal.style.display = "none";
            submitBtn.removeEventListener("click", handleSubmit);
            inputEl.removeEventListener("keydown", keyHandler);
            onConfirm(val);
        }
    };

    const keyHandler = (e) => {
        if (e.key === "Enter") handleSubmit();
        if (e.key === "Escape") closePromptDialog();
    };

    submitBtn.onclick = handleSubmit;
    inputEl.onkeydown = keyHandler;
}

function closePromptDialog() {
    const modal = document.getElementById("prompt-modal");
    if (modal) modal.style.display = "none";
}

function showConfirmDialog({ title, message, confirmText = "Confirm", danger = true, onConfirm }) {
    const modal = document.getElementById("confirm-modal");
    const titleEl = document.getElementById("confirm-title");
    const msgEl = document.getElementById("confirm-message");
    const btn = document.getElementById("confirm-action-btn");

    if (!modal || !btn) return;

    titleEl.innerText = title;
    msgEl.innerText = message;
    btn.innerText = confirmText;
    btn.className = danger ? "btn btn-danger" : "btn btn-primary";

    modal.style.display = "flex";

    btn.onclick = () => {
        modal.style.display = "none";
        onConfirm();
    };
}

function closeConfirmDialog() {
    const modal = document.getElementById("confirm-modal");
    if (modal) modal.style.display = "none";
}

// --- CRUD Actions ---
function createFolderPrompt() {
    showPromptDialog({
        title: "Create Virtual Folder",
        label: "Folder Name",
        placeholder: "e.g., Documents, Work",
        confirmText: "Create Folder",
        onConfirm: async (name) => {
            try {
                const res = await fetch("/api/folders", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ name, parent_id: currentFolderId })
                });
                if (!res.ok) throw new Error("Failed to create folder");
                showToast(`Folder "${name}" created`, "success");
                loadDriveContent();
            } catch (err) {
                showToast(err.message, "error");
            }
        }
    });
}

function promptRenameFolder(id, currentName) {
    showPromptDialog({
        title: "Rename Virtual Folder",
        label: "New Folder Name",
        defaultValue: currentName,
        confirmText: "Rename",
        onConfirm: async (newName) => {
            if (newName === currentName) return;
            try {
                const res = await fetch(`/api/folders/${id}`, {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ name: newName })
                });
                if (!res.ok) throw new Error("Failed to rename folder");
                showToast("Folder renamed", "success");
                loadDriveContent();
            } catch (err) {
                showToast(err.message, "error");
            }
        }
    });
}

function confirmDeleteFolder(id, name) {
    showConfirmDialog({
        title: "Delete Virtual Folder",
        message: `Are you sure you want to delete "${name}" and all of its contents? This cannot be undone.`,
        confirmText: "Delete Folder",
        danger: true,
        onConfirm: async () => {
            try {
                const res = await fetch(`/api/folders/${id}`, { method: "DELETE" });
                if (!res.ok) throw new Error("Failed to delete folder");
                showToast(`Folder "${name}" deleted`, "success");
                loadDriveContent();
            } catch (err) {
                showToast(err.message, "error");
            }
        }
    });
}

function promptRenameFile(id, currentName) {
    showPromptDialog({
        title: "Rename Virtual File",
        label: "New File Name",
        defaultValue: currentName,
        confirmText: "Rename",
        onConfirm: async (newName) => {
            if (newName === currentName) return;
            try {
                const res = await fetch(`/api/files/${id}`, {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ name: newName })
                });
                if (!res.ok) throw new Error("Failed to rename file");
                showToast("File renamed", "success");
                loadDriveContent();
            } catch (err) {
                showToast(err.message, "error");
            }
        }
    });
}

function confirmDeleteFile(id, name) {
    showConfirmDialog({
        title: "Delete Virtual File",
        message: `Are you sure you want to delete "${name}"?`,
        confirmText: "Delete File",
        danger: true,
        onConfirm: async () => {
            try {
                const res = await fetch(`/api/files/${id}`, { method: "DELETE" });
                if (!res.ok) throw new Error("Failed to delete file");
                showToast(`File "${name}" deleted`, "success");
                loadDriveContent();
            } catch (err) {
                showToast(err.message, "error");
            }
        }
    });
}

function downloadFile(id, name) {
    window.location.href = `/api/files/${id}/download`;
}

// --- Utilities ---
function formatSize(bytes) {
    if (!bytes || bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

function formatDate(dateStr) {
    if (!dateStr) return "";
    const d = new Date(dateStr);
    return d.toLocaleDateString(undefined, {
        month: "short",
        day: "numeric",
        year: "numeric"
    });
}

function formatDateTime(dateStr) {
    if (!dateStr) return "";
    const d = new Date(dateStr);
    return d.toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit"
    });
}

function escapeHtml(str) {
    if (!str) return "";
    return String(str)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        showToast("Copied to clipboard!", "success");
    }).catch(() => {
        showToast("Failed to copy", "error");
    });
}

// --- Mobile Sidebar Setup ---
function setupMobileSidebar() {
    const toggleBtn = document.getElementById("mobile-menu-btn");
    const sidebar = document.getElementById("sidebar");
    const backdrop = document.getElementById("sidebar-backdrop");

    if (toggleBtn && sidebar && backdrop) {
        toggleBtn.addEventListener("click", () => {
            sidebar.classList.toggle("open");
            backdrop.classList.toggle("open");
        });
        backdrop.addEventListener("click", () => {
            sidebar.classList.remove("open");
            backdrop.classList.remove("open");
        });
    }
}

// --- Selection State & Bulk Actions ---
function isSelected(type, id) {
    return selectedItems.has(`${type}:${id}`);
}

function toggleSelectItem(type, id, event) {
    if (event) event.stopPropagation();
    const key = `${type}:${id}`;
    if (selectedItems.has(key)) {
        selectedItems.delete(key);
    } else {
        let name = "";
        if (type === "folder") {
            const f = cachedFolders.find(x => x.id === id);
            if (f) name = f.name;
        } else {
            const f = cachedFiles.find(x => x.id === id);
            if (f) name = f.name;
        }
        selectedItems.set(key, { id, type, name });
    }
    updateSelectionUI();
}

function selectAll() {
    cachedFolders.forEach(f => selectedItems.set(`folder:${f.id}`, { id: f.id, type: "folder", name: f.name }));
    cachedFiles.forEach(f => selectedItems.set(`file:${f.id}`, { id: f.id, type: "file", name: f.name }));
    updateSelectionUI();
}

function deselectAll() {
    selectedItems.clear();
    updateSelectionUI();
}

function toggleSelectAll(event) {
    if (event) event.stopPropagation();
    const totalItems = cachedFolders.length + cachedFiles.length;
    if (totalItems === 0) return;
    if (selectedItems.size === totalItems) {
        deselectAll();
    } else {
        selectAll();
    }
}

function updateSelectionUI() {
    document.querySelectorAll(".card, tr").forEach(el => {
        const id = el.getAttribute("data-id");
        const type = el.getAttribute("data-type");
        if (!id || !type) return;
        const checked = isSelected(type, id);
        el.classList.toggle("selected", checked);
        const cb = el.querySelector(".item-checkbox");
        if (cb && !cb.id) cb.classList.toggle("checked", checked);
    });

    const totalItems = cachedFolders.length + cachedFiles.length;
    const selectAllCb = document.getElementById("select-all-checkbox");
    if (selectAllCb) {
        selectAllCb.classList.toggle("checked", totalItems > 0 && selectedItems.size === totalItems);
    }

    const bar = document.getElementById("bulk-action-bar");
    const countEl = document.getElementById("bulk-selected-count");
    if (bar && countEl) {
        countEl.innerText = selectedItems.size;
        bar.style.display = selectedItems.size > 0 ? "flex" : "none";
    }
}

async function batchTrashSelected() {
    if (selectedItems.size === 0) return;
    const count = selectedItems.size;

    showConfirmDialog({
        title: "Move to Virtual Trash",
        message: `Are you sure you want to move ${count} item(s) to Virtual Trash?`,
        confirmText: "Trash Items",
        confirmClass: "btn-danger",
        onConfirm: async () => {
            const fileIds = [];
            const folderIds = [];
            selectedItems.forEach(item => {
                if (item.type === "folder") folderIds.push(item.id);
                else fileIds.push(item.id);
            });

            try {
                const res = await fetch("/api/batch/trash", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ file_ids: fileIds, folder_ids: folderIds })
                });
                if (!res.ok) throw new Error("Failed to trash items");
                deselectAll();
                await loadDriveContent();
                showToast(`Moved ${count} item(s) to Virtual Trash`, "success");
            } catch (err) {
                showToast(err.message, "error");
            }
        }
    });
}

// --- Move Modal & Tree Picker ---
let moveTargets = []; // array of { id, type, name }
let selectedDestinationFolderId = null;

async function openMoveModal(id, type, name) {
    moveTargets = [{ id, type, name }];
    const titleEl = document.getElementById("move-modal-title");
    if (titleEl) titleEl.innerText = `Move "${name}"`;
    selectedDestinationFolderId = null;
    await renderMoveFolderTree();
    const modal = document.getElementById("move-modal");
    if (modal) modal.style.display = "flex";
}

async function openBatchMoveModal() {
    if (selectedItems.size === 0) return;
    moveTargets = Array.from(selectedItems.values());
    const titleEl = document.getElementById("move-modal-title");
    if (titleEl) titleEl.innerText = `Move ${moveTargets.length} item(s)`;
    selectedDestinationFolderId = null;
    await renderMoveFolderTree();
    const modal = document.getElementById("move-modal");
    if (modal) modal.style.display = "flex";
}

function closeMoveModal() {
    const modal = document.getElementById("move-modal");
    if (modal) modal.style.display = "none";
    moveTargets = [];
    selectedDestinationFolderId = null;
}

async function renderMoveFolderTree() {
    const container = document.getElementById("move-folder-tree");
    if (!container) return;
    container.innerHTML = '<div style="color: var(--text-muted); font-size: 0.85rem; padding: 8px;">Loading folders...</div>';

    try {
        const res = await fetch("/api/folders");
        const folders = await res.json() || [];
        const targetFolderIds = new Set(moveTargets.filter(t => t.type === "folder").map(t => t.id));

        let html = `
            <div class="tree-folder-node ${selectedDestinationFolderId === null ? "selected" : ""}" onclick="selectMoveDestination(null, this)">
                ${getIcon("folder", "icon-sm")}
                <span>Drive (Root)</span>
            </div>
        `;

        folders.filter(f => !targetFolderIds.has(f.id)).forEach(f => {
            html += `
                <div class="tree-folder-node ${selectedDestinationFolderId === f.id ? "selected" : ""}" onclick="selectMoveDestination('${f.id}', this)" style="margin-left: 16px;">
                    ${getIcon("folder", "icon-sm")}
                    <span>${escapeHtml(f.name)}</span>
                </div>
            `;
        });

        container.innerHTML = html;
    } catch (err) {
        container.innerHTML = `<div style="color: var(--danger); font-size: 0.85rem; padding: 8px;">Failed to load folders</div>`;
    }
}

function selectMoveDestination(folderId, element) {
    selectedDestinationFolderId = folderId;
    document.querySelectorAll(".tree-folder-node").forEach(el => el.classList.remove("selected"));
    if (element) element.classList.add("selected");
}

async function confirmMoveDestination() {
    if (moveTargets.length === 0) return;
    const fileIds = moveTargets.filter(t => t.type === "file").map(t => t.id);
    const folderIds = moveTargets.filter(t => t.type === "folder").map(t => t.id);

    try {
        const res = await fetch("/api/batch/move", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                file_ids: fileIds,
                folder_ids: folderIds,
                target_folder_id: selectedDestinationFolderId
            })
        });
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || "Move failed");
        }
        closeMoveModal();
        deselectAll();
        await loadDriveContent();
        showToast("Moved successfully", "success");
    } catch (err) {
        showToast(err.message, "error");
    }
}

// --- Drag & Drop for Moving Items ---
function handleDragStart(e, id, type) {
    draggedItem = { id, type };
    e.dataTransfer.setData("text/plain", `${type}:${id}`);
    e.dataTransfer.effectAllowed = "move";
    if (e.currentTarget) e.currentTarget.classList.add("dragging");
}

function handleDragEnd(e) {
    if (e.currentTarget) e.currentTarget.classList.remove("dragging");
    document.querySelectorAll(".drop-hover").forEach(el => el.classList.remove("drop-hover"));
    draggedItem = null;
}

function handleDragOver(e) {
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer.dropEffect = "move";
    if (e.currentTarget && !e.currentTarget.classList.contains("drop-hover")) {
        e.currentTarget.classList.add("drop-hover");
    }
}

function handleDragLeave(e) {
    if (e.currentTarget) e.currentTarget.classList.remove("drop-hover");
}

async function handleDropOnTarget(e, targetFolderId) {
    e.preventDefault();
    e.stopPropagation();
    if (e.currentTarget) e.currentTarget.classList.remove("drop-hover");
    if (!draggedItem) return;

    if (draggedItem.type === "folder" && draggedItem.id === targetFolderId) {
        return;
    }

    let fileIds = [];
    let folderIds = [];

    if (isSelected(draggedItem.type, draggedItem.id) && selectedItems.size > 1) {
        selectedItems.forEach(item => {
            if (item.type === "folder") folderIds.push(item.id);
            else fileIds.push(item.id);
        });
    } else {
        if (draggedItem.type === "folder") folderIds.push(draggedItem.id);
        else fileIds.push(draggedItem.id);
    }

    try {
        const res = await fetch("/api/batch/move", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                file_ids: fileIds,
                folder_ids: folderIds,
                target_folder_id: targetFolderId || null
            })
        });
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || "Move failed");
        }
        deselectAll();
        await loadDriveContent();
        showToast("Moved successfully", "success");
    } catch (err) {
        showToast(err.message, "error");
    }
}

// --- Keyboard Shortcuts ---
document.addEventListener("keydown", (e) => {
    const active = document.activeElement;
    if (active && (active.tagName === "INPUT" || active.tagName === "TEXTAREA" || active.tagName === "SELECT")) return;
    const anyModalOpen = Array.from(document.querySelectorAll(".modal-overlay")).some(m => m.style.display === "flex");
    if (anyModalOpen) return;

    if (e.key === "F2") {
        if (selectedItems.size === 1) {
            e.preventDefault();
            const item = Array.from(selectedItems.values())[0];
            if (item.type === "folder") promptRenameFolder(item.id, item.name);
            else promptRenameFile(item.id, item.name);
        }
    } else if (e.key === "Delete" || e.key === "Backspace") {
        if (selectedItems.size > 0) {
            e.preventDefault();
            batchTrashSelected();
        }
    }
});

// --- Changelog Modal Viewer ---
async function openChangelogModal() {
    const modal = document.getElementById("changelog-modal");
    const body = document.getElementById("changelog-body");
    const verTag = document.getElementById("changelog-version-tag");
    if (!modal || !body) return;

    modal.style.display = "flex";
    body.innerHTML = "<p style='color: var(--text-muted);'>Loading changelog...</p>";

    try {
        const res = await fetch("/api/changelog");
        if (!res.ok) throw new Error("Could not load changelog");
        const data = await res.json();
        const ver = data.version || "1.5.0";
        if (verTag) verTag.innerText = `Installed: v${ver}`;
        body.innerHTML = renderChangelog(data.content, ver);
    } catch (err) {
        body.innerHTML = `<p style="color: var(--danger);">Failed to load changelog: ${escapeHtml(err.message)}</p>`;
    }
}

function closeChangelogModal() {
    const modal = document.getElementById("changelog-modal");
    if (modal) modal.style.display = "none";
}

function renderChangelog(text, currentVersion) {
    if (!text) return "<p style='color: var(--text-muted);'>No changelog available.</p>";

    const lines = text.split("\n");
    const releases = [];
    let currentRelease = null;
    let currentSection = null;

    for (let rawLine of lines) {
        const line = rawLine.trimEnd();
        const verMatch = line.match(/^##\s+\[?([0-9a-zA-Z.-]+)\]?(?:\s*-\s*(\d{4}-\d{2}-\d{2}))?/);
        if (verMatch) {
            currentRelease = {
                version: verMatch[1],
                date: verMatch[2] || "",
                sections: []
            };
            releases.push(currentRelease);
            currentSection = null;
            continue;
        }

        const secMatch = line.match(/^###\s+(.+)$/);
        if (secMatch && currentRelease) {
            currentSection = {
                title: secMatch[1].trim(),
                items: []
            };
            currentRelease.sections.push(currentSection);
            continue;
        }

        if (currentSection) {
            currentSection.items.push(rawLine);
        }
    }

    if (releases.length === 0) {
        return `<div class="changelog-raw">${escapeHtml(text)}</div>`;
    }

    const curVerClean = (currentVersion || "").replace(/^v/, "").trim();

    let out = '<div class="changelog-timeline">';
    for (const rel of releases) {
        const relVerClean = rel.version.replace(/^v/, "").trim();
        const isCurrent = curVerClean && (relVerClean === curVerClean);

        out += `<div class="changelog-card ${isCurrent ? "is-current" : ""}">`;
        out += `  <div class="changelog-card-header">`;
        out += `    <div class="changelog-ver-wrapper">`;
        out += `      <span class="changelog-ver-badge">v${escapeHtml(relVerClean)}</span>`;
        if (isCurrent) {
            out += `      <span class="changelog-status-badge">Current Installed</span>`;
        }
        out += `    </div>`;
        if (rel.date) {
            out += `    <span class="changelog-date">${escapeHtml(rel.date)}</span>`;
        }
        out += `  </div>`;

        out += `  <div class="changelog-card-body">`;
        for (const sec of rel.sections) {
            out += `    <div class="changelog-section">`;
            out += `      <div class="changelog-section-header">${getCategoryBadge(sec.title)}</div>`;
            out += `      ${renderChangelogItems(sec.items)}`;
            out += `    </div>`;
        }
        out += `  </div>`;
        out += `</div>`;
    }
    out += '</div>';
    return out;
}

function getCategoryBadge(category) {
    const cat = (category || "").toLowerCase();
    let badgeClass = "badge-added";
    if (cat.includes("change")) badgeClass = "badge-changed";
    else if (cat.includes("fix")) badgeClass = "badge-fixed";
    else if (cat.includes("remove") || cat.includes("deprecat")) badgeClass = "badge-removed";
    else if (cat.includes("security")) badgeClass = "badge-security";
    return `<span class="changelog-cat-badge ${badgeClass}">${escapeHtml(category)}</span>`;
}

function formatChangelogInline(str) {
    let s = escapeHtml(str);
    s = s.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/`([^`]+)`/g, '<code class="inline-code" style="background: var(--bg-surface-active, rgba(255,255,255,0.06)); padding: 2px 6px; border-radius: 4px; font-family: var(--font-mono); font-size: 0.85em;">$1</code>');
    s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer" class="changelog-link">$1</a>');
    return s;
}

function renderChangelogItems(lines) {
    let html = '<ul class="changelog-items">';
    let inSubList = false;

    for (let raw of lines) {
        const trimmed = raw.trim();
        if (!trimmed || trimmed === "---") continue;

        const subMatch = raw.match(/^\s{2,4}-\s+(.+)$/);
        const topMatch = raw.match(/^-\s+(.+)$/);

        if (subMatch) {
            if (!inSubList) {
                html += '<ul class="changelog-subitems">';
                inSubList = true;
            }
            html += `<li>${formatChangelogInline(subMatch[1])}</li>`;
        } else if (topMatch) {
            if (inSubList) {
                html += '</ul>';
                inSubList = false;
            }
            html += `<li class="changelog-item">${formatChangelogInline(topMatch[1])}</li>`;
        } else {
            if (inSubList) {
                html += '</ul>';
                inSubList = false;
            }
            html += `<p class="changelog-text">${formatChangelogInline(trimmed)}</p>`;
        }
    }
    if (inSubList) {
        html += '</ul>';
    }
    html += '</ul>';
    return html;
}

// --- Auto-Update System ---
let latestReleaseData = null;

async function checkSystemUpdate() {
    try {
        const res = await fetch("/api/system/update");
        if (!res.ok) return;
        const data = await res.json();
        if (data.update_available && data.release) {
            latestReleaseData = data;
            const badge = document.getElementById("sidebar-update-badge");
            const badgeText = document.getElementById("sidebar-update-text");
            if (badge) badge.style.display = "block";
            if (badgeText) badgeText.innerText = `Update to ${data.latest_version}`;
        }
    } catch (_) {
        // Silent fallback when offline
    }
}

function openUpdateModal() {
    const modal = document.getElementById("update-modal");
    if (!modal) return;

    if (latestReleaseData) {
        const curVer = document.getElementById("update-current-ver");
        const latVer = document.getElementById("update-latest-ver");
        const notes = document.getElementById("update-release-notes");
        if (curVer) curVer.innerText = `v${latestReleaseData.current_version}`;
        if (latVer) latVer.innerText = latestReleaseData.latest_version;
        if (notes && latestReleaseData.release) {
            notes.innerText = latestReleaseData.release.body || "No release notes provided.";
        }
    }

    const content = document.getElementById("update-modal-content");
    const footer = document.getElementById("update-modal-footer");
    const progress = document.getElementById("update-progress-container");
    if (content) content.style.display = "block";
    if (footer) footer.style.display = "flex";
    if (progress) progress.style.display = "none";

    modal.style.display = "flex";
}

function closeUpdateModal() {
    const modal = document.getElementById("update-modal");
    if (modal) modal.style.display = "none";
}

async function applyUpdate() {
    const content = document.getElementById("update-modal-content");
    const footer = document.getElementById("update-modal-footer");
    const progress = document.getElementById("update-progress-container");
    const statusText = document.getElementById("update-status-text");

    if (content) content.style.display = "none";
    if (footer) footer.style.display = "none";
    if (progress) progress.style.display = "block";

    try {
        const res = await fetch("/api/system/update", {
            method: "POST",
            headers: { "Content-Type": "application/json" }
        });
        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || "Update failed to apply");
        }

        if (statusText) {
            statusText.innerText = "Update applied successfully! Reconnecting to TeleDrive...";
        }

        let attempts = 0;
        const pollInterval = setInterval(async () => {
            attempts++;
            try {
                const ping = await fetch("/api/changelog?t=" + Date.now());
                if (ping.ok) {
                    clearInterval(pollInterval);
                    showToast("TeleDrive successfully updated! Reloading...", "success");
                    setTimeout(() => window.location.reload(), 1200);
                }
            } catch (_) {
                if (attempts > 30) {
                    clearInterval(pollInterval);
                    if (statusText) statusText.innerText = "Update applied. Please refresh the page manually.";
                }
            }
        }, 1500);

    } catch (err) {
        if (progress) progress.style.display = "none";
        if (content) content.style.display = "block";
        if (footer) footer.style.display = "flex";
        showToast("Update failed: " + err.message, "error");
    }
}

// --- Telegram MTProto Session & Account Status ---
async function loadTelegramStatus() {
    const dot = document.getElementById("tg-status-dot");
    const title = document.getElementById("tg-status-title");
    const accountLine = document.getElementById("tg-account-line");
    const channelLine = document.getElementById("tg-channel-line");
    const disconnectBox = document.getElementById("tg-disconnect-container");
    const banner = document.getElementById("telegram-disconnected-banner");

    if (!dot || !title) return;

    try {
        const res = await fetch("/api/system/telegram");
        if (!res.ok) throw new Error("Failed to query Telegram status");
        const data = await res.json();

        if (data.authorized) {
            dot.style.background = "var(--success, #22c55e)";
            dot.style.boxShadow = "0 0 8px rgba(34, 197, 94, 0.4)";
            title.innerText = "MTProto Online";

            let userStr = "Connected";
            if (data.phone) {
                userStr = "+" + data.phone;
            } else if (data.username) {
                userStr = "@" + data.username;
            } else if (data.first_name) {
                userStr = data.first_name;
            }
            if (accountLine) accountLine.innerHTML = `<strong>Account:</strong> ${escapeHtml(userStr)}`;
            if (channelLine) channelLine.innerHTML = `<strong>Channel:</strong> ${data.channel_id ? "ID " + data.channel_id : "Connected"}`;
            if (disconnectBox) disconnectBox.style.display = "block";
            if (banner) banner.style.display = "none";
        } else {
            dot.style.background = "var(--danger, #ef4444)";
            dot.style.boxShadow = "0 0 8px rgba(239, 68, 68, 0.4)";
            title.innerText = "Telegram Disconnected";
            if (accountLine) accountLine.innerHTML = `<strong>Account:</strong> Unlinked`;
            if (channelLine) channelLine.innerHTML = `<strong>Channel:</strong> None`;
            if (disconnectBox) disconnectBox.style.display = "none";
            if (banner) banner.style.display = "flex";
        }
    } catch (_) {
        dot.style.background = "var(--warning, #f59e0b)";
        title.innerText = "Status Unavailable";
    }
}

async function confirmDisconnectTelegram() {
    const confirmed = confirm(
        "Are you sure you want to disconnect Telegram from TeleDrive?\n\n" +
        "• The MTProto session will be revoked.\n" +
        "• Virtual files and folders in SQLite will be preserved.\n" +
        "• To resume file uploads or downloads, you will need to run 'teledrive login' in your terminal."
    );
    if (!confirmed) return;

    try {
        const res = await fetch("/api/system/telegram/disconnect", {
            method: "POST",
            headers: { "Content-Type": "application/json" }
        });
        if (!res.ok) {
            const err = await res.text();
            throw new Error(err || "Failed to disconnect Telegram");
        }
        showToast("Telegram account disconnected successfully.", "info");
        await loadTelegramStatus();
    } catch (err) {
        showToast("Disconnect error: " + err.message, "error");
    }
}

