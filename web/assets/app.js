// DOM Elements
const searchForm = document.getElementById('search-form');
const startPathInput = document.getElementById('startPath');
const namePatternInput = document.getElementById('namePattern');
const contentPatternInput = document.getElementById('contentPattern');
const searchArchivesCheckbox = document.getElementById('searchArchives');
const ignoreCaseCheckbox = document.getElementById('ignoreCase');
const useRegexCheckbox = document.getElementById('useRegex');

const btnSearch = document.getElementById('btn-search');
const btnCancel = document.getElementById('btn-cancel');
const btnClear = document.getElementById('btn-clear');
const btnBrowseCurrent = document.getElementById('btn-browse-current');
const btnBrowseFolder = document.getElementById('btn-browse-folder');
const btnExportCSV = document.getElementById('btn-export-csv');

const statStatus = document.getElementById('stat-status');
const statStatusSub = document.getElementById('stat-status-sub');
const statMatches = document.getElementById('stat-matches');
const statMatchesSub = document.getElementById('stat-matches-sub');
const statTime = document.getElementById('stat-time');
const statTimeSub = document.getElementById('stat-time-sub');

const resultsFilter = document.getElementById('results-filter');
const progressBar = document.getElementById('search-progress');
const noResultsState = document.getElementById('no-results');
const resultsTable = document.getElementById('results-table');
const resultsTbody = document.getElementById('results-tbody');
const platformBadge = document.getElementById('platform-badge');
const toast = document.getElementById('toast');

// Archive Format Config Elements
const archiveFormatsContainer = document.getElementById('archive-formats-container');
const formatsList = document.getElementById('formats-list');
const newExtInput = document.getElementById('new-ext');
const newTypeSelect = document.getElementById('new-type');
const btnAddFormat = document.getElementById('btn-add-format');

// Search State Variables
let eventSource = null;
let startTime = null;
let timerInterval = null;
let matchCount = 0;
let resultsList = []; // store results for local filtering

const defaultArchiveFormats = [
    { extension: '.zip', type: 'zip', enabled: true },
    { extension: '.jar', type: 'zip', enabled: true },
    { extension: '.war', type: 'zip', enabled: true },
    { extension: '.tar', type: 'tar', enabled: true },
    { extension: '.tgz', type: 'tar', enabled: true },
    { extension: '.tar.gz', type: 'tar', enabled: true },
    { extension: '.gz', type: 'tar', enabled: true }
];

let archiveFormats = [];

function loadArchiveFormats() {
    const saved = localStorage.getItem('deepfind_archive_formats');
    if (saved) {
        try {
            archiveFormats = JSON.parse(saved);
        } catch (e) {
            archiveFormats = JSON.parse(JSON.stringify(defaultArchiveFormats));
        }
    } else {
        archiveFormats = JSON.parse(JSON.stringify(defaultArchiveFormats));
    }
}

function saveArchiveFormats() {
    localStorage.setItem('deepfind_archive_formats', JSON.stringify(archiveFormats));
}

function renderArchiveFormats() {
    formatsList.innerHTML = '';
    
    archiveFormats.forEach((fmt, index) => {
        const item = document.createElement('div');
        item.className = 'format-item';
        
        item.innerHTML = `
            <div class="format-item-left">
                <label class="checkbox-label" style="margin: 0;">
                    <input type="checkbox" class="format-checkbox" data-index="${index}" ${fmt.enabled ? 'checked' : ''}>
                    <span class="checkbox-custom"></span>
                    <span class="format-item-label">${escapeHTML(fmt.extension)}</span>
                </label>
                <span class="format-type-badge ${fmt.type}">${escapeHTML(fmt.type)}</span>
            </div>
            <button type="button" class="btn-remove-format" data-index="${index}" title="Remove format">&times;</button>
        `;
        
        // Listen to checkbox changes
        const checkbox = item.querySelector('.format-checkbox');
        checkbox.addEventListener('change', (e) => {
            archiveFormats[index].enabled = e.target.checked;
            saveArchiveFormats();
        });
        
        // Listen to remove button
        const btnRemove = item.querySelector('.btn-remove-format');
        btnRemove.addEventListener('click', () => {
            archiveFormats.splice(index, 1);
            saveArchiveFormats();
            renderArchiveFormats();
        });
        
        formatsList.appendChild(item);
    });
}

// Initialize Platform badge and defaults
window.addEventListener('DOMContentLoaded', () => {
    fetchPlatform();
    // Default search path is current directory
    startPathInput.value = '.';
    
    // Load and render archive formats
    loadArchiveFormats();
    renderArchiveFormats();
    
    // Toggle container based on checkbox state
    searchArchivesCheckbox.addEventListener('change', () => {
        if (searchArchivesCheckbox.checked) {
            archiveFormatsContainer.style.display = 'flex';
        } else {
            archiveFormatsContainer.style.display = 'none';
        }
    });
    
    // Trigger initial state
    if (searchArchivesCheckbox.checked) {
        archiveFormatsContainer.style.display = 'flex';
    }
    
    // Add format event listener
    btnAddFormat.addEventListener('click', () => {
        let ext = newExtInput.value.trim().toLowerCase();
        if (!ext) return;
        if (!ext.startsWith('.')) {
            ext = '.' + ext;
        }
        
        // Prevent duplicates
        if (archiveFormats.some(f => f.extension === ext)) {
            alert('Extension already exists!');
            return;
        }
        
        const type = newTypeSelect.value;
        archiveFormats.push({ extension: ext, type: type, enabled: true });
        saveArchiveFormats();
        renderArchiveFormats();
        
        newExtInput.value = '';
    });

    // Directory Browser Event Listener
    btnBrowseFolder.addEventListener('click', async () => {
        try {
            const res = await fetch('/api/browse');
            if (res.ok) {
                const data = await res.json();
                if (data.path) {
                    startPathInput.value = data.path;
                }
            } else {
                const errText = await res.text();
                console.error('Error selecting folder:', errText);
            }
        } catch (err) {
            console.error('Fetch error selecting folder:', err);
        }
    });
});

// Fetch Server Platform
async function fetchPlatform() {
    try {
        const res = await fetch('/api/platform');
        if (res.ok) {
            const data = await res.json();
            platformBadge.textContent = `OS: ${data.os} (${data.arch})`;
            if (data.currentDir) {
                startPathInput.placeholder = `Current: ${data.currentDir}`;
            }
        } else {
            platformBadge.textContent = 'OS: Connected';
        }
    } catch (err) {
        platformBadge.textContent = 'OS: Disconnected';
        platformBadge.style.color = 'var(--accent-rose)';
    }
}

// Current Dir shortcut button
btnBrowseCurrent.addEventListener('click', () => {
    startPathInput.value = '.';
});

// Export CSV button
btnExportCSV.addEventListener('click', async () => {
    if (!resultsList || resultsList.length === 0) {
        showToast('No results to export', 'warning');
        return;
    }
    
    try {
        const response = await fetch('/api/export-csv', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(resultsList)
        });
        
        if (!response.ok) {
            const errText = await response.text();
            showToast(`Export failed: ${response.statusText} - ${errText}`, 'error');
            return;
        }
        
        // Trigger download
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = 'deepfind-results.csv';
        link.click();
        window.URL.revokeObjectURL(url);
        
        showToast(`Exported ${resultsList.length} results to CSV`, 'success');
    } catch (err) {
        console.error('CSV export error:', err);
        showToast(`Export error: ${err.message}`, 'error');
    }
});


// Search execution
searchForm.addEventListener('submit', (e) => {
    e.preventDefault();
    startSearch();
});

btnCancel.addEventListener('click', () => {
    cancelSearch('Cancelled');
});

btnClear.addEventListener('click', () => {
    clearResults();
});

function startSearch() {
    // 1. Reset UI & State
    cancelSearch('Resetting');
    matchCount = 0;
    resultsList = [];
    resultsTbody.innerHTML = '';
    statMatches.textContent = '0';
    statMatchesSub.textContent = '0 files matched';
    
    noResultsState.style.display = 'none';
    resultsTable.style.display = 'table';
    resultsFilter.disabled = false;
    resultsFilter.value = '';
    
    statStatus.textContent = 'Searching';
    statStatus.className = 'metric-value searching';
    statStatusSub.textContent = 'Scanning directory tree...';
    
    progressBar.style.width = '0%';
    progressBar.className = 'progress-bar active'; // triggers infinite sweep
    
    btnSearch.disabled = true;
    btnCancel.disabled = false;

    // 2. Start timer
    startTime = Date.now();
    timerInterval = setInterval(() => {
        const elapsed = (Date.now() - startTime) / 1000;
        statTime.textContent = elapsed.toFixed(2) + 's';
        statTimeSub.textContent = `Streaming results`;
    }, 50);

    // 3. Build query parameters
    const params = new URLSearchParams({
        startPath: startPathInput.value.trim(),
        namePattern: namePatternInput.value.trim(),
        contentPattern: contentPatternInput.value.trim(),
        searchArchives: searchArchivesCheckbox.checked,
        archiveFormats: JSON.stringify(archiveFormats),
        ignoreCase: ignoreCaseCheckbox.checked,
        useRegex: useRegexCheckbox.checked
    });

    // 4. Setup EventSource for SSE
    const url = `/api/search?${params.toString()}`;
    eventSource = new EventSource(url);

    eventSource.onmessage = (event) => {
        // Handle done signal
        if (event.data === '[DONE]') {
            finishSearch('Completed', 'Search completed successfully');
            return;
        }
        
        // Parse and render results
        try {
            const result = JSON.parse(event.data);
            if (result.error) {
                // If there's an error on a specific item, we can render it
                appendErrorRow(result);
            } else {
                matchCount++;
                statMatches.textContent = matchCount;
                statMatchesSub.textContent = `${matchCount} matches found`;
                resultsList.push(result);
                appendResultRow(result);
            }
        } catch (err) {
            console.error('Error parsing SSE event:', err);
        }
    };

    eventSource.onerror = (err) => {
        console.error('SSE Error:', err);
        // SSE connection closed or disconnected
        finishSearch('Error', 'Connection closed with backend');
    };
}

function cancelSearch(statusText = 'Cancelled') {
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }
    stopTimer();
    btnSearch.disabled = false;
    btnCancel.disabled = true;
    progressBar.className = 'progress-bar';
    progressBar.style.width = '0%';
    
    if (statusText !== 'Resetting') {
        statStatus.textContent = statusText;
        statStatus.className = `metric-value ${statusText.toLowerCase()}`;
        statStatusSub.textContent = 'Search stopped';
    }
}

function finishSearch(status, subtext) {
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }
    stopTimer();
    btnSearch.disabled = false;
    btnCancel.disabled = true;
    progressBar.className = 'progress-bar';
    progressBar.style.width = '100%';
    
    statStatus.textContent = status;
    statStatus.className = `metric-value ${status.toLowerCase()}`;
    statStatusSub.textContent = subtext;
}

function stopTimer() {
    if (timerInterval) {
        clearInterval(timerInterval);
        timerInterval = null;
    }
}

function clearResults() {
    cancelSearch('Idle');
    matchCount = 0;
    resultsList = [];
    resultsTbody.innerHTML = '';
    statMatches.textContent = '0';
    statMatchesSub.textContent = '0 files matched';
    statTime.textContent = '0.00s';
    statTimeSub.textContent = 'Real-time update';
    statStatus.textContent = 'Idle';
    statStatus.className = 'metric-value idle';
    statStatusSub.textContent = 'Ready to search';
    
    noResultsState.style.display = 'flex';
    resultsTable.style.display = 'none';
    resultsFilter.value = '';
    resultsFilter.disabled = true;
}

// Render normal result row
function appendResultRow(res) {
    const tr = document.createElement('tr');
    
    // Path Column
    const tdPath = document.createElement('td');
    tdPath.className = 'cell-path';
    
    let pathHTML = '';
    if (res.isArchive) {
        // Show archive package details
        const archiveName = getBasename(res.archivePath);
        pathHTML = `
            <div class="path-main">${res.innerPath}</div>
            <div class="path-archive">
                <span class="path-archive-tag">Archive</span> 
                ${archiveName} &rarr; ${res.archivePath}
            </div>
        `;
    } else {
        pathHTML = `<div class="path-main">${res.path}</div>`;
    }
    tdPath.innerHTML = pathHTML;

    // Type Column
    const tdType = document.createElement('td');
    if (res.isArchive) {
        tdType.innerHTML = `<span class="badge-type archive">Archive Entry</span>`;
    } else {
        tdType.innerHTML = `<span class="badge-type file">File</span>`;
    }

    // Details Column (matching content line)
    const tdDetails = document.createElement('td');
    tdDetails.className = 'cell-details';
    if (res.isContentMatch) {
        // Format code block
        const highlighted = escapeHTML(res.lineContent);
        // Try to highlight match
        const finalHTML = highlightContent(highlighted, contentPatternInput.value, useRegexCheckbox.checked, ignoreCaseCheckbox.checked);
        
        tdDetails.innerHTML = `
            <div class="match-line-meta">Line ${res.lineNumber}</div>
            <div class="match-line-content"><code>${finalHTML}</code></div>
        `;
    } else {
        tdDetails.innerHTML = `<span style="color: var(--text-muted);">Filename matched</span>`;
    }

    // Actions Column (Copy Button)
    const tdActions = document.createElement('td');
    tdActions.className = 'cell-actions';
    
    const copyPath = res.isArchive ? `${res.archivePath}!/${res.innerPath}` : res.path;
    
    const btnCopy = document.createElement('button');
    btnCopy.className = 'btn-icon';
    btnCopy.title = 'Copy Path';
    btnCopy.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>`;
    btnCopy.addEventListener('click', () => copyToClipboard(copyPath));
    
    tdActions.appendChild(btnCopy);

    tr.appendChild(tdPath);
    tr.appendChild(tdType);
    tr.appendChild(tdDetails);
    tr.appendChild(tdActions);
    
    resultsTbody.appendChild(tr);
    
    // Auto-scroll list to bottom during search if active
    if (eventSource) {
        const wrapper = document.querySelector('.results-list-wrapper');
        wrapper.scrollTop = wrapper.scrollHeight;
    }
}

// Render error row
function appendErrorRow(res) {
    const tr = document.createElement('tr');
    tr.style.backgroundColor = 'rgba(244, 63, 94, 0.05)';
    
    const tdPath = document.createElement('td');
    tdPath.className = 'cell-path';
    tdPath.innerHTML = `<div class="path-main" style="color: var(--accent-rose);">${res.path || 'System'}</div>`;
    
    const tdType = document.createElement('td');
    tdType.innerHTML = `<span class="badge-type" style="background-color: rgba(244,63,94,0.15); color: var(--accent-rose);">Error</span>`;
    
    const tdDetails = document.createElement('td');
    tdDetails.className = 'cell-details';
    tdDetails.innerHTML = `<div style="color: var(--accent-rose); font-family: var(--font-mono); font-size: 0.8rem;">${escapeHTML(res.error)}</div>`;
    
    const tdActions = document.createElement('td');
    
    tr.appendChild(tdPath);
    tr.appendChild(tdType);
    tr.appendChild(tdDetails);
    tr.appendChild(tdActions);
    resultsTbody.appendChild(tr);
}

// Copy value to clipboard
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        toast.className = 'toast show';
        setTimeout(() => {
            toast.className = 'toast';
        }, 2000);
    }).catch(err => {
        console.error('Failed to copy text: ', err);
    });
}

// Filtering displayed rows
resultsFilter.addEventListener('input', (e) => {
    const query = e.target.value.toLowerCase().trim();
    const rows = resultsTbody.querySelectorAll('tr');
    
    rows.forEach(row => {
        const text = row.textContent.toLowerCase();
        if (text.includes(query)) {
            row.style.display = '';
        } else {
            row.style.display = 'none';
        }
    });
});

// Helper: Escape HTML string
function escapeHTML(str) {
    if (!str) return '';
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

// Helper: highlight content query
function highlightContent(text, pattern, isRegex, ignoreCase) {
    if (!pattern) return text;
    
    try {
        let regex;
        if (isRegex) {
            regex = new RegExp(`(${pattern})`, ignoreCase ? 'gi' : 'g');
        } else {
            // Escape special regex chars for plain substring highlight
            const escaped = pattern.replace(/[-\/\\^$*+?.()|[\]{}]/g, '\\$&');
            regex = new RegExp(`(${escaped})`, ignoreCase ? 'gi' : 'g');
        }
        return text.replace(regex, '<mark>$1</mark>');
    } catch (e) {
        // fallback to unhighlighted if regex is invalid
        return text;
    }
}

// Helper: get base name
function getBasename(path) {
    if (!path) return '';
    return path.split(/[/\\]/).pop();
}

