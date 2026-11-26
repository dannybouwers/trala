/**
 * TraLa Application JavaScript
 */
const API_URL = '/api/services';
// Defaults. These will be overridden by frontend config fetch
let SEARCH_ENGINE_URL = 'https://www.google.com/search?q=';
let SEARCH_ENGINE_ICON_URL = '';
let REFRESH_INTERVAL_SECONDS = 30;
let GROUPING_COLUMNS = 3;

// Translation strings are loaded from data attributes on the body element
const getTranslation = (key) => document.body.dataset[key] || '';

// Constants for hardcoded strings and CSS classes
const GRID_CLASSES_UNGROUPED = 'grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4 md:gap-6';

// HTML escaping function to prevent XSS attacks
const escapeHtml = (unsafe) => {
    if (unsafe === null || unsafe === undefined) {
        return '';
    }
    return String(unsafe)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
};

const serviceGrid = document.getElementById('service-grid');
const searchInput = document.getElementById('search-input');
const clearButton = document.getElementById('clear-button');
const sortControls = document.getElementById('sort-controls');
const searchForm = document.getElementById('search-form');
const searchButton = document.getElementById('search-button');
const searchIcon = document.getElementById('search-icon');
const searchIconFallback = document.getElementById('search-icon-fallback');
const apiLoadingBar = document.getElementById('api-loading-bar');
const refreshProgressBar = document.getElementById('refresh-progress-bar');
const errorPage = document.getElementById('error-page');
const errorMessage = document.getElementById('error-message');
const greetingText = document.getElementById('greeting-text');
const clock = document.getElementById('clock');
const configWarning = document.getElementById('config-warning');
const groupControls = document.getElementById('group-controls');
const groupingButtons = document.getElementById('group-buttons');
const groupToggle = document.getElementById('group-toggle');
const expandCollapseAll = document.getElementById('expand-collapse-all');
const multiHostBottons = document.getElementById('multi-host-buttons');
const mixToggle = document.getElementById('mix-toggle');
const minimizedControls = document.getElementById('minimized-controls');

let allServices = [];
let allExpanded = true;
let refreshIntervalId = null;
let currentSort = 'name';
let groupingEnabled = false; // Will be set after fetching server config
let multiHost = false;
let mixServices = false;
const colors = ['bg-red-500', 'bg-orange-500', 'bg-amber-500', 'bg-yellow-500', 'bg-lime-500', 'bg-green-500', 'bg-emerald-500', 'bg-teal-500', 'bg-cyan-500', 'bg-sky-500', 'bg-blue-500', 'bg-indigo-500', 'bg-violet-500', 'bg-purple-500', 'bg-fuchsia-500', 'bg-pink-500', 'bg-rose-500'];

const getColorFromString = (str) => { let hash = 0; for (let i = 0; i < str.length; i++) { hash = str.charCodeAt(i) + ((hash << 5) - hash); } return colors[Math.abs(hash % colors.length)]; };

// Function to generate dynamic grid classes for grouped view based on column count
const getGroupedGridClasses = (columns) => {
    // Clamp the value between 1 and 6
    columns = Math.max(1, Math.min(6, columns));
    
    // Generate responsive grid classes
    let gridClasses = 'grid gap-4';
    
    // Always 1 column on mobile
    gridClasses += ' grid-cols-1';
    
    // Add medium screen size only if columns > 1, never more than xl columns
    if (columns > 1) {
        gridClasses += ' md:grid-cols-2';
    }
    
    // Add xl screen size with the configured number of columns
    gridClasses += ` xl:grid-cols-${columns}`;
    
    return gridClasses;
};

// Function to get card grid classes based on group columns
const getCardGridClasses = (groupColumns) => {
    // Clamp the value between 1 and 6
    groupColumns = Math.max(1, Math.min(6, groupColumns));

    // Calculate card columns: total cards should be 6 on xl, so cardColumns = 6 / groupColumns
    const cardColumns = Math.floor(6 / groupColumns);
    
    // Always 2 columns on mobile and medium screens
    return `group-content grid grid-cols-2 xl:grid-cols-${cardColumns} gap-4`;
};
const setApiLoading = (isLoading) => { apiLoadingBar.classList.toggle('loading', isLoading); };
const showErrorPage = (message) => { serviceGrid.classList.add('hidden'); sortControls.classList.add('hidden'); groupControls.classList.add('hidden'); errorPage.classList.remove('hidden'); errorMessage.textContent = message; renderMinimizedPills(); };
const hideErrorPage = () => { serviceGrid.classList.remove('hidden'); sortControls.classList.remove('hidden'); groupControls.classList.remove('hidden'); errorPage.classList.add('hidden'); renderMinimizedPills(); };

const updateGreeting = () => {
    const hour = new Date().getHours();
    let greeting;
    if (hour < 6) {
        greeting = getTranslation('greetingNight');
    } else if (hour < 12) {
        greeting = getTranslation('greetingMorning');
    } else if (hour < 18) {
        greeting = getTranslation('greetingAfternoon');
    } else {
        greeting = getTranslation('greetingEvening');
    }
    greetingText.textContent = greeting;
};

const formatClock = (date = new Date()) => date.toLocaleTimeString(navigator.language, { hour: 'numeric', minute: '2-digit' });

const updateClock = () => {
    const time = formatClock();
    if (clock) clock.textContent = time;
    document.querySelectorAll('.ui-restore-pill-clock').forEach((el) => { el.textContent = time; });
};

// --- Minimizable dashboard blocks ---

const MINIMIZED_STORAGE_KEY = 'uiMinimized';
const MINIMIZE_CLOSE_ICON = '<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 6l12 12M6 18L18 6"/></svg>';
const MINIMIZABLE_BLOCKS = [
    { key: 'greeting', id: 'greeting-header', clock: true, labelKey: 'restore' },
    { key: 'search', id: 'search-container', icon: '<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>', labelKey: 'search' },
    { key: 'sort', id: 'sort-controls', icon: '<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6h16M7 12h10M10 18h4"/></svg>', labelKey: 'sort' },
    { key: 'group', id: 'group-controls', icon: '<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2 2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>', labelKey: 'groups' },
];

const minimizedKeys = new Set();

const isBlockConfigHidden = (block) =>
    !block || block.classList.contains('hidden') || block.style.display === 'none';

const readMinimizedKeys = () => {
    try {
        const raw = localStorage.getItem(MINIMIZED_STORAGE_KEY);
        if (!raw) return [];
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return [];
        return parsed.filter((key) => MINIMIZABLE_BLOCKS.some((config) => config.key === key));
    } catch (error) {
        return [];
    }
};

const persistMinimizedKeys = () => {
    try {
        localStorage.setItem(MINIMIZED_STORAGE_KEY, JSON.stringify([...minimizedKeys]));
    } catch (error) {
        // localStorage unavailable (e.g. Safari private mode); keep per-load behaviour only.
    }
};

const renderMinimizedPills = () => {
    if (!minimizedControls) return;
    minimizedControls.innerHTML = '';
    MINIMIZABLE_BLOCKS.forEach((config) => {
        if (!minimizedKeys.has(config.key)) return;
        const block = document.getElementById(config.id);
        if (isBlockConfigHidden(block)) return;

        const pill = document.createElement('button');
        pill.type = 'button';
        pill.className = 'ui-restore-pill inline-flex items-center justify-center gap-1.5 min-w-10 h-10 px-3 rounded-lg bg-gray-50 dark:bg-gray-800 text-gray-700 dark:text-gray-300 cursor-pointer shadow-md transition-all duration-300 hover:shadow-lg hover:-translate-y-1 focus-visible:ring-2 focus-visible:ring-blue-500';
        pill.dataset.target = config.key;
        const label = getTranslation(config.labelKey);
        pill.title = label;
        pill.setAttribute('aria-label', label);

        if (config.clock) {
            const timeSpan = document.createElement('span');
            timeSpan.className = 'ui-restore-pill-clock px-1 font-medium tabular-nums';
            timeSpan.textContent = formatClock();
            pill.appendChild(timeSpan);
        } else {
            pill.innerHTML = config.icon;
        }

        pill.addEventListener('click', () => restoreBlock(config.key));
        minimizedControls.appendChild(pill);
    });
    const isEmpty = minimizedControls.children.length === 0;
    minimizedControls.classList.toggle('hidden', isEmpty);
    minimizedControls.classList.toggle('flex', !isEmpty);
};

const minimizeBlock = (key) => {
    const config = MINIMIZABLE_BLOCKS.find((entry) => entry.key === key);
    if (!config) return;
    const block = document.getElementById(config.id);
    if (isBlockConfigHidden(block) || block.classList.contains('ui-minimized')) return;
    block.classList.add('ui-minimized');
    minimizedKeys.add(key);
    renderMinimizedPills();
    persistMinimizedKeys();
};

const restoreBlock = (key) => {
    const config = MINIMIZABLE_BLOCKS.find((entry) => entry.key === key);
    if (!config) return;
    const block = document.getElementById(config.id);
    if (block) block.classList.remove('ui-minimized');
    minimizedKeys.delete(key);
    renderMinimizedPills();
    persistMinimizedKeys();
    if (key === 'search' && searchInput) searchInput.focus();
};

const initMinimize = () => {
    MINIMIZABLE_BLOCKS.forEach((config) => {
        const block = document.getElementById(config.id);
        if (!block || block.querySelector('.ui-minimize-btn')) return;
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'ui-minimize-btn absolute -top-2 -right-2 z-10 flex h-[22px] w-[22px] items-center justify-center rounded-full border border-gray-300 bg-white text-gray-600 opacity-0 scale-90 pointer-events-none transition duration-150 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700 group-hover:opacity-100 group-hover:scale-100 group-hover:pointer-events-auto group-focus-within:opacity-100 group-focus-within:scale-100 group-focus-within:pointer-events-auto focus-visible:ring-2 focus-visible:ring-blue-500';
        button.title = getTranslation('minimize');
        button.setAttribute('aria-label', getTranslation('minimize'));
        button.innerHTML = MINIMIZE_CLOSE_ICON;
        button.addEventListener('click', () => minimizeBlock(config.key));
        block.appendChild(button);
    });

    readMinimizedKeys().forEach((key) => {
        minimizedKeys.add(key);
        const config = MINIMIZABLE_BLOCKS.find((entry) => entry.key === key);
        const block = config && document.getElementById(config.id);
        if (block) block.classList.add('ui-minimized');
    });

    renderMinimizedPills();
};


const startRefreshBarAnimation = () => {
    refreshProgressBar.style.transition = 'none';
    refreshProgressBar.style.width = '0%';
    setTimeout(() => {
        refreshProgressBar.style.transition = `width ${REFRESH_INTERVAL_SECONDS}s linear`;
        refreshProgressBar.style.width = '100%';
    }, 50);
};

const createServiceCard = (service) => {
    const card = document.createElement('a');
    card.href = service.url;
    card.target = '_blank';
    card.rel = 'noopener noreferrer';
    card.className = 'service-tile block p-4 rounded-lg bg-white dark:bg-gray-800 shadow-md hover:shadow-lg hover:-translate-y-1 transition-all duration-300';

    const firstLetter = service.Name.charAt(0).toUpperCase();
    const bgColor = getColorFromString(service.Name);

    card.innerHTML = `<div class="flex flex-col items-center text-center"><div class="w-16 h-16 mb-4 flex items-center justify-center rounded-lg overflow-hidden"><img class="w-full h-full object-contain icon-img" src="${escapeHtml(service.icon)}" alt="Icon for ${escapeHtml(service.Name)}" style="display: block;" /><div class="fallback-icon w-full h-full ${bgColor}" style="display: none;">${escapeHtml(firstLetter)}</div></div><p class="font-semibold truncate w-full" title="${escapeHtml(service.Name)}">${escapeHtml(service.Name)}</p><p class="text-xs text-gray-500 dark:text-gray-400 truncate w-full" title="${escapeHtml(service.url)}">${escapeHtml(service.url.replace('https://', ''))}</p></div>`;

    const img = card.querySelector('.icon-img');
    const fallback = card.querySelector('.fallback-icon');

    if (service.icon) {
        img.onerror = () => {
            img.style.display = 'none';
            fallback.style.display = 'flex';
        };
    } else {
        img.style.display = 'none';
        fallback.style.display = 'flex';
    }

    return card;
};

// In ungrouped mode, services are displayed in a single flat grid
const renderUngroupedView = (servicesToRender, container = serviceGrid) => {
    container.className = GRID_CLASSES_UNGROUPED;
    container.innerHTML = '';
    if (servicesToRender.length === 0 && searchInput.value) { container.innerHTML = `<p class="col-span-full text-center text-gray-500 dark:text-gray-400">No services found for "${escapeHtml(searchInput.value)}".</p>`; return; }

    for (const service of servicesToRender) {
        const card = createServiceCard(service);
        container.appendChild(card);
    }
};

// In multi-host mode, services are grouped by host, each with a collapsible header, and reuse the mixed view rendering
const renderHostView = (servicesToRender) => {
    serviceGrid.className = '';
    serviceGrid.innerHTML = '';
    if (servicesToRender.length === 0) {
        serviceGrid.innerHTML = searchInput.value ? `<p class="text-center text-gray-500 dark:text-gray-400">No services found for "${escapeHtml(searchInput.value)}".</p>` : '';
        return;
    }

    const grouped = servicesToRender.reduce((acc, service) => {
        const FALLBACK_HOST_NAME = getTranslation('unknown');
        const host = service.host || FALLBACK_HOST_NAME;
        if (!acc[host]) acc[host] = [];
        acc[host].push(service);
        return acc;
    }, {});

    const sortedHosts = Object.keys(grouped).sort();
    sortedHosts.forEach(host => {
        const hostDiv = document.createElement('div');
        hostDiv.className = 'host-container mb-8';
        const header = document.createElement('h2');
        header.className = 'text-2xl font-bold mb-4 cursor-pointer border-b border-gray-300 dark:border-gray-700 pb-2';
        header.textContent = host;
        const content = document.createElement('div');
        content.className = 'host-content';
        content.style.display = allExpanded ? 'block' : 'none';
        let hostExpanded = true;
        header.addEventListener('click', () => {
            hostExpanded = !hostExpanded;
            content.style.display = hostExpanded ? 'block' : 'none';
        });
        hostDiv.appendChild(header);
        hostDiv.appendChild(content);
        serviceGrid.appendChild(hostDiv);

        const inner = document.createElement('div');
        content.appendChild(inner);
        renderMixedView(grouped[host], inner);
    });
};

// In grouped mode, services are organized into collapsible sections with headers, each containing a grid of services
const renderGroupedView = (servicesToRender, container = serviceGrid) => {
    container.className = getGroupedGridClasses(GROUPING_COLUMNS);
    if (servicesToRender.length === 0) {
        container.innerHTML = searchInput.value ? `<p class="text-center text-gray-500 dark:text-gray-400">No services found for "${escapeHtml(searchInput.value)}".</p>` : '';
        return;
    }
    container.innerHTML = '';
    const FALLBACK_GROUP_NAME = getTranslation('uncategorized');
    const grouped = servicesToRender.reduce((acc, service) => {
        const group = service.group || FALLBACK_GROUP_NAME;
        if (!acc[group]) acc[group] = [];
        acc[group].push(service);
        return acc;
    }, {});
    const sortedGroups = Object.keys(grouped).sort();
    sortedGroups.forEach(group => {
        const groupDiv = document.createElement('div');
        groupDiv.className = 'group-container mb-8';
        const header = document.createElement('h2');
        header.className = 'text-xl font-bold mb-4 cursor-pointer border-b border-gray-300 dark:border-gray-700 pb-2';
        header.textContent = group;
        let isExpanded = true;
        header.addEventListener('click', () => {
            isExpanded = !isExpanded;
            content.style.display = isExpanded ? 'grid' : 'none';
        });
        groupDiv.appendChild(header);
        const content = document.createElement('div');
        content.className = getCardGridClasses(GROUPING_COLUMNS);
        content.style.display = allExpanded ? 'grid' : 'none';
        grouped[group].sort((a, b) => b.priority - a.priority).forEach(service => {
            const card = createServiceCard(service);
            content.appendChild(card);
        });
        groupDiv.appendChild(content);
        container.appendChild(groupDiv);
    });
};

const renderMixedView = (servicesToRender, container = serviceGrid) => {
    if (!groupingEnabled) {
        renderUngroupedView(servicesToRender, container);
    } else {
        renderGroupedView(servicesToRender, container);
    }
};

const renderServices = (servicesToRender) => {
    if (multiHost && !mixServices) {
        renderHostView(servicesToRender);
    } else {
        renderMixedView(servicesToRender, serviceGrid);
    }
};

const applyFiltersAndSort = () => {
    const searchTerm = searchInput.value.toLowerCase();
    let filteredServices = allServices.filter(service => service.Name.toLowerCase().includes(searchTerm) || service.url.toLowerCase().includes(searchTerm));
    
    let sortedServices = [...filteredServices];
    switch (currentSort) {
        case 'name': 
            sortedServices.sort((a, b) => a.Name.localeCompare(b.Name)); 
            break;
        case 'url': 
            sortedServices.sort((a, b) => a.url.localeCompare(b.url)); 
            break;
        case 'priority': 
            sortedServices.sort((a, b) => b.priority - a.priority); 
            break;
    }
    renderServices(sortedServices);
};

const fetchAndProcessServices = async () => {
    setApiLoading(true);
    hideErrorPage();
    try {
        const response = await fetch(API_URL);
        if (!response.ok) { 
            const errorText = await response.text();
            throw new Error(`API request failed: ${response.status} - ${errorText}`); 
        }
        let data = await response.json();
        if (!Array.isArray(data)) { 
            showErrorPage("Invalid data from API."); 
            allServices = []; 
        } else { 
            const currentHref = window.location.href.replace(/\/$/, ""); 
            allServices = data.filter(service => {
                const serviceHref = service.url.replace(/\/$/, "");
                return serviceHref !== currentHref;
            });
        }
        applyFiltersAndSort();
    } catch (error) {
        console.error("Error fetching services:", error);
        showErrorPage(error.message);
        allServices = [];
    } finally {
        setApiLoading(false);
    }
};

const initialize = () => {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)');
    const applyTheme = (isDark) => { document.documentElement.classList.toggle('dark', isDark); };
    applyTheme(prefersDark.matches);
    prefersDark.addEventListener('change', (e) => applyTheme(e.matches));
    
    searchInput.addEventListener('input', () => {
        // Show/hide clear button based on input content
        clearButton.style.display = searchInput.value ? 'block' : 'none';
        applyFiltersAndSort();
    });
    
    // Clear search when clear button is clicked
    clearButton.addEventListener('click', () => {
        searchInput.value = '';
        clearButton.style.display = 'none';
        applyFiltersAndSort();
        searchInput.focus();
    });
    
    sortControls.addEventListener('click', (e) => {
        const sortButton = e.target.closest('button.sort-btn');
        if (!sortButton) return;
        currentSort = sortButton.dataset.sort;
        document.querySelectorAll('.sort-btn').forEach(btn => btn.classList.remove('active'));
        sortButton.classList.add('active');
        applyFiltersAndSort();
    });

    groupToggle.addEventListener('click', () => {
        groupingEnabled = !groupingEnabled;
        localStorage.setItem('groupingEnabled', groupingEnabled);
        groupToggle.classList.toggle('active');
        if (groupingEnabled) allExpanded = true;
        applyFiltersAndSort();
    });

    if (mixToggle) {
        mixToggle.addEventListener('click', () => {
            mixServices = !mixServices;
            localStorage.setItem('mixServices', mixServices);
            mixToggle.classList.toggle('active', mixServices);
            applyFiltersAndSort();
        });
    }

    expandCollapseAll.addEventListener('click', () => {
        allExpanded = !allExpanded;
        document.querySelectorAll('.group-content').forEach(content => {
            content.style.display = allExpanded ? 'grid' : 'none';
        });
        document.querySelectorAll('.host-content').forEach(content => {
            content.style.display = allExpanded ? 'block' : 'none';
        });
    });

    searchForm.addEventListener('submit', (e) => { e.preventDefault(); if (searchInput.value) { window.open(`${SEARCH_ENGINE_URL}${encodeURIComponent(searchInput.value)}`, '_blank'); } });
    

    // Fetch all application status information in a single call
    const fetchApplicationStatus = async () => {
        try {
            const response = await fetch('/api/status');
            if (!response.ok) {
                throw new Error(`Status request failed: ${response.status}`);
            }
            
            const status = await response.json();
            
            // Update version information
            const versionElement = document.getElementById('version-number');
            const versionLink = document.getElementById('version-link');
            if (versionElement && versionLink && status.version) {
                const version = status.version.version || 'unknown';
                versionElement.textContent = version;
                versionLink.href = `https://github.com/dannybouwers/trala/releases/tag/${version}`;
            }
            
            // Update configuration status warning
            if (status.config && configWarning) {
                // Show warning if config is incompatible OR if there's a warning message
                if (!status.config.isCompatible || status.config.warningMessage) {
                    configWarning.style.display = 'inline';
                    configWarning.title = status.config.warningMessage || 'Configuration issue detected';
                }
            }
            
            // Update frontend configuration
            if (status.frontend) {
                SEARCH_ENGINE_URL = status.frontend.searchEngineURL || SEARCH_ENGINE_URL;
                SEARCH_ENGINE_ICON_URL = status.frontend.searchEngineIconURL || '';
                REFRESH_INTERVAL_SECONDS = status.frontend.refreshIntervalSeconds || REFRESH_INTERVAL_SECONDS;

                // Update search icon if available
                if (SEARCH_ENGINE_ICON_URL) {
                    searchIcon.src = SEARCH_ENGINE_ICON_URL;
                    searchIcon.className = 'w-5 h-5 opacity-70 search-icon-greyscale';
                    searchIcon.style.display = 'block';
                    searchIconFallback.style.display = 'none';

                    // Handle icon load error
                    searchIcon.onerror = () => {
                        searchIcon.style.display = 'none';
                        searchIconFallback.style.display = 'block';
                    };
                }

                if (status.frontend.groupingEnabled || status.frontend.multiHost) {
                    groupControls.style.display = 'flex';
                }

                // Update grouping configuration
                if (status.frontend.groupingEnabled !== undefined) {
                    groupingEnabled = status.frontend.groupingEnabled;
                    groupingButtons.style.display = status.frontend.groupingEnabled ? '' : 'none';
                    const storedGrouping = localStorage.getItem('groupingEnabled');
                    if (storedGrouping !== null && groupingEnabled) {
                        groupingEnabled = storedGrouping === 'true';
                    }
                    groupToggle.classList.toggle('active', groupingEnabled);
                }
                
                // Update grouped columns configuration
                if (status.frontend.groupingColumns !== undefined) {
                    GROUPING_COLUMNS = status.frontend.groupingColumns;
                }

                // Update multi-host configuration
                if (status.frontend.multiHost !== undefined) {
                    multiHost = status.frontend.multiHost;
                    multiHostBottons.style.display = multiHost ? '' : 'none';
                    const storedMix = localStorage.getItem('mixServices');
                    if (storedMix !== null) {
                        mixServices = storedMix === 'true';
                    }
                    mixToggle.classList.toggle('active', mixServices);
                }
            }
            
            return status;
        } catch (error) {
            console.error('Error fetching application status:', error);
            
            // Set fallback values
            const versionElement = document.getElementById('version-number');
            const versionLink = document.getElementById('version-link');
            if (versionElement && versionLink) {
                versionElement.textContent = 'dev';
                versionLink.href = 'https://github.com/dannybouwers/trala/releases';
            }
            
            return null;
        }
    };

    const startApp = async () => {
        // Fetch all application status information in a single call
        await fetchApplicationStatus();

        initMinimize();

        updateGreeting();
        updateClock();
        setInterval(() => {
            updateClock();
            updateGreeting();
        }, 6000);

        await fetchAndProcessServices();
        if (refreshIntervalId) clearInterval(refreshIntervalId);
        if (!isNaN(REFRESH_INTERVAL_SECONDS) && REFRESH_INTERVAL_SECONDS > 0) {
            startRefreshBarAnimation();
            refreshIntervalId = setInterval(async () => {
                await fetchAndProcessServices();
                startRefreshBarAnimation();
            }, REFRESH_INTERVAL_SECONDS * 1000);
        }
    };
    
    startApp();
};

initialize();
