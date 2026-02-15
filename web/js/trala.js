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
const groupToggle = document.getElementById('group-toggle');
const groupControls = document.getElementById('group-controls');
const expandCollapseAll = document.getElementById('expand-collapse-all');

let allServices = [];
let allExpanded = true;
let refreshIntervalId = null;
let currentSort = 'name';
let groupingEnabled = false; // Will be set after fetching server config
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
const showErrorPage = (message) => { serviceGrid.classList.add('hidden'); sortControls.classList.add('hidden'); groupControls.classList.add('hidden'); errorPage.classList.remove('hidden'); errorMessage.textContent = message; };
const hideErrorPage = () => { serviceGrid.classList.remove('hidden'); sortControls.classList.remove('hidden'); groupControls.classList.remove('hidden'); errorPage.classList.add('hidden'); };

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

const updateClock = () => {
    const now = new Date();
    clock.textContent = now.toLocaleTimeString(navigator.language, { hour: 'numeric', minute: '2-digit' });
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
    card.className = 'block p-4 rounded-lg bg-white dark:bg-gray-800 shadow-md hover:shadow-lg hover:-translate-y-1 transition-all duration-300';

    const firstLetter = service.Name.charAt(0).toUpperCase();
    const bgColor = getColorFromString(service.Name);

    card.innerHTML = `<div class="flex flex-col items-center text-center"><div class="w-16 h-16 mb-4 flex items-center justify-center rounded-lg overflow-hidden"><img class="w-full h-full object-contain icon-img" src="${service.icon}" alt="Icon for ${service.Name}" style="display: block;" /><div class="fallback-icon w-full h-full ${bgColor}" style="display: none;">${firstLetter}</div></div><p class="font-semibold truncate w-full" title="${service.Name}">${service.Name}</p><p class="text-xs text-gray-500 dark:text-gray-400 truncate w-full" title="${service.url}">${service.url.replace('https://', '')}</p></div>`;

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
const renderUngroupedView = (servicesToRender) => {
    serviceGrid.className = GRID_CLASSES_UNGROUPED;
    serviceGrid.innerHTML = '';
    if (servicesToRender.length === 0 && searchInput.value) { serviceGrid.innerHTML = `<p class="col-span-full text-center text-gray-500 dark:text-gray-400">No services found for "${searchInput.value}".</p>`; return; }

    for (const service of servicesToRender) {
        const card = createServiceCard(service);
        serviceGrid.appendChild(card);
    }
};

// In grouped mode, services are organized into collapsible sections with headers, each containing a grid of services
const renderGroupedView = (servicesToRender) => {
    serviceGrid.className = getGroupedGridClasses(GROUPING_COLUMNS);
    if (servicesToRender.length === 0) {
        serviceGrid.innerHTML = searchInput.value ? `<p class="text-center text-gray-500 dark:text-gray-400">No services found for "${searchInput.value}".</p>` : '';
        return;
    }
    serviceGrid.innerHTML = '';
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
        groupDiv.className = 'group-section mb-8';
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
        serviceGrid.appendChild(groupDiv);
    });
};

const renderServices = (servicesToRender) => {
    if (!groupingEnabled) {
        renderUngroupedView(servicesToRender);
    } else {
        renderGroupedView(servicesToRender);
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
        if (e.target.tagName === 'BUTTON') {
            currentSort = e.target.dataset.sort;
            document.querySelectorAll('.sort-btn').forEach(btn => btn.classList.remove('active'));
            e.target.classList.add('active');
            applyFiltersAndSort();
        }
    });

    groupToggle.addEventListener('click', () => {
        groupingEnabled = !groupingEnabled;
        localStorage.setItem('groupingEnabled', groupingEnabled);
        groupToggle.classList.toggle('active');
        if (groupingEnabled) allExpanded = true;
        applyFiltersAndSort();
    });

    expandCollapseAll.addEventListener('click', () => {
        allExpanded = !allExpanded;
        const groupContents = document.querySelectorAll('.group-content');
        groupContents.forEach(content => {
            content.style.display = allExpanded ? 'grid' : 'none';
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

                // Update grouping configuration
                if (status.frontend.groupingEnabled !== undefined) {
                    groupingEnabled = status.frontend.groupingEnabled;
                    groupControls.style.display = status.frontend.groupingEnabled ? 'flex' : 'none';
                    // Load persisted toggle state if available
                    const storedGrouping = localStorage.getItem('groupingEnabled');
                    if (storedGrouping !== null) {
                        groupingEnabled = storedGrouping === 'true';
                    }
                    groupToggle.classList.toggle('active', groupingEnabled);
                }
                
                // Update grouped columns configuration
                if (status.frontend.groupingColumns !== undefined) {
                    GROUPING_COLUMNS = status.frontend.groupingColumns;
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
