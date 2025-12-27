// Avoid re-declaration error with HTMX
if (typeof currentEventIdForSettings === 'undefined') {
    var currentEventIdForSettings = null;
}
if (typeof currentEventLicenseMode === 'undefined') {
    var currentEventLicenseMode = null;
}

async function refreshEvents() {
  const container = document.getElementById('events-list')
  if (!container) return

  const [eventsRes, meRes] = await Promise.all([
      authService.fetchWithAuth(authService.apiUrl + '/events'),
      authService.fetchWithAuth(authService.apiUrl + '/users/me')
  ]);

  if (!eventsRes.ok) {
    return
  }
  const events = await eventsRes.json()
  const me = meRes.ok ? await meRes.json() : null;
  const currentUserId = me ? me.userId : null;
  
  container.innerHTML = ''
  
  if (!events || events.length === 0) {
    container.innerHTML = '<p class="text-text-muted py-4 text-center">No events found.</p>';
    return;
  }

  events.forEach(ev => {
    const item = document.createElement('div')
    item.setAttribute('data-id', ev.id)
    item.className = 'relative flex justify-between items-center p-4 bg-white/5 rounded-md hover:bg-white/10 transition-colors'
    
    // Create stretched link
    const link = document.createElement('a')
    link.href = '/events/' + ev.id
    link.className = 'absolute inset-0'
    item.appendChild(link)

    const info = document.createElement('div')
    info.className = 'flex-1 min-w-0 pr-4 pointer-events-none'
    
    const name = document.createElement('div')
    name.className = 'font-bold text-text truncate'
    name.textContent = decodeHTMLEntities(ev.name)
    info.appendChild(name)

    const vis = ev.visibility.charAt(0).toUpperCase() + ev.visibility.slice(1);
    const lic = ev.licenseMode === 'invited_only' ? 'Invited Only' : 
                ev.licenseMode === 'geo_time' ? 'Location/Time' : 
                (ev.licenseMode ? ev.licenseMode.charAt(0).toUpperCase() + ev.licenseMode.slice(1) : 'Everyone');
    
    const meta = document.createElement('div')
    meta.className = 'text-sm text-text-muted truncate'
    meta.textContent = `${vis} • License: ${lic}`
    info.appendChild(meta)
    
    item.appendChild(info)

    // Only show actions if current user is owner
    if (currentUserId && ev.ownerId === currentUserId) {
        const actions = document.createElement('div')
        actions.className = 'flex items-center gap-4 relative z-10'
        
        // Rename Button
        const btnRename = document.createElement('button')
        btnRename.className = 'btn-small'
        btnRename.textContent = 'Rename'
        btnRename.onclick = (e) => {
            e.preventDefault();
            renameEvent(ev.id, ev.name)
        }
        actions.appendChild(btnRename)

        // Delete Button (Trash Icon)
        const btnDelete = document.createElement('button')
        btnDelete.className = 'inline-flex items-center justify-center text-primary hover:text-primary-hover transition-colors'
        btnDelete.title = 'Delete'
        btnDelete.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
        `
        btnDelete.onclick = (e) => {
            e.preventDefault();
            deleteEvent(ev.id)
        }
        actions.appendChild(btnDelete)

        item.appendChild(actions)
    }

    container.appendChild(item)
  })

  if (window.htmx) {
      window.htmx.process(container);
  }
}

async function loadEvent(id) {
    const res = await authService.fetchWithAuth(authService.apiUrl + '/events/' + id)
    if (!res.ok) {
        window.showAlert({ title: 'Error', content: 'Failed to load event.' })
        return
    }
    const ev = await res.json()
    
    currentEventLicenseMode = ev.licenseMode;
    
    document.getElementById('ev-name').textContent = decodeHTMLEntities(ev.name)
    
    const vis = ev.visibility.charAt(0).toUpperCase() + ev.visibility.slice(1);
    const lic = ev.licenseMode === 'invited_only' ? 'Invited Only' : 
                ev.licenseMode === 'geo_time' ? 'Location/Time' : 
                (ev.licenseMode ? ev.licenseMode.charAt(0).toUpperCase() + ev.licenseMode.slice(1) : 'Everyone');

    document.getElementById('ev-meta').textContent = `${vis} • License: ${lic}`

    // Check ownership
    const meRes = await authService.fetchWithAuth(authService.apiUrl + '/users/me');
    if (meRes.ok) {
        const me = await meRes.json();
        if (me.userId === ev.ownerId) {
            const btn = document.getElementById('ev-settings-btn');
            if (btn) btn.classList.remove('hidden');
            
            // Show start round button for owner
            const roundBtn = document.getElementById('btn-start-round');
            if (roundBtn) roundBtn.style.display = 'inline-block';
        } else {
            // Ensure hidden for non-owners
            const roundBtn = document.getElementById('btn-start-round');
            if (roundBtn) roundBtn.style.display = 'none';
        }
    }
    
    checkActiveRound();
}

window.openEventSettings = async function() {
    const id = document.getElementById('ev-id').textContent;
    currentEventIdForSettings = id;

    const res = await authService.fetchWithAuth(authService.apiUrl + '/events/' + id);
    const ev = await res.json();

    const isPrivate = ev.visibility === 'private';
    const licenseMode = ev.licenseMode || 'everyone';
    const showInvites = isPrivate || licenseMode === 'invited_only';
    const showGeo = licenseMode === 'geo_time';

    // Create Modal Content
    const content = `
      <div class="space-y-4 text-left">
        <div>
           <label class="block text-sm font-medium text-text-muted mb-1">Visibility (Who can see/join)</label>
           <select id="setting-visibility" onchange="updateSettingsUI()" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-text">
             <option value="public" ${ev.visibility === 'public' ? 'selected' : ''}>Public</option>
             <option value="private" ${ev.visibility === 'private' ? 'selected' : ''}>Private</option>
           </select>
           <p class="text-xs text-text-muted mt-1">Private: Only invited users can find and vote.</p>
        </div>

        <div>
           <label class="block text-sm font-medium text-text-muted mb-1">Voting License (Who can vote)</label>
           <select id="setting-license" onchange="updateSettingsUI()" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-text">
             <option value="everyone" ${licenseMode === 'everyone' ? 'selected' : ''}>Everyone</option>
             <option value="invited_only" ${licenseMode === 'invited_only' ? 'selected' : ''}>Invited Only</option>
             <option value="geo_time" ${licenseMode === 'geo_time' ? 'selected' : ''}>Location/Time (License)</option>
           </select>
        </div>
        
        <div id="settings-geo-section" class="border-t border-input-border pt-4" style="display: ${showGeo ? 'block' : 'none'};">
            <h4 class="text-md font-medium mb-2">Location Requirements</h4>
            <div class="grid grid-cols-2 gap-2 mb-2">
                <div>
                    <label class="text-xs text-text-muted">Latitude</label>
                    <input id="setting-geo-lat" type="number" step="any" value="${ev.geoLat || ''}" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 text-text" />
                </div>
                <div>
                    <label class="text-xs text-text-muted">Longitude</label>
                    <input id="setting-geo-lng" type="number" step="any" value="${ev.geoLng || ''}" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 text-text" />
                </div>
            </div>
            <div class="mb-2">
                <label class="text-xs text-text-muted">Radius (meters)</label>
                <input id="setting-geo-radius" type="number" value="${ev.geoRadiusM || 100}" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 text-text" />
            </div>
            <button onclick="setEventLocationFromDevice()" class="btn-small w-full flex items-center justify-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M5.05 4.05a7 7 0 119.9 9.9L10 18.9l-4.95-4.95a7 7 0 010-9.9zM10 11a2 2 0 100-4 2 2 0 000 4z" clip-rule="evenodd" />
                </svg>
                Set to My Location
            </button>
        </div>

        <div id="settings-invites-section" class="border-t border-input-border pt-4" style="display: ${showInvites ? 'block' : 'none'};">
           <h4 class="text-md font-medium mb-2">Invites</h4>
           <div class="flex gap-2 mb-2">
             <input id="invite-user-id" placeholder="User ID to invite" class="flex-1 bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-sm text-text" />
             <button onclick="sendInvite()" class="btn-small">Invite</button>
           </div>
           <ul id="settings-invites-list" class="max-h-40 overflow-y-auto space-y-1 text-sm text-text-muted">
             <li>Loading invites...</li>
           </ul>
        </div>
      </div>
    `;

    window.showModal({
        title: 'Event Settings',
        content: content,
        buttons: [
            { text: 'Save Changes', class: 'px-4 py-2 bg-white/10 hover:bg-white/20 text-text rounded-md transition-colors', onclick: saveEventSettings },
            { text: 'Close', class: 'px-4 py-2 bg-white/5 hover:bg-white/10 text-text-muted rounded-md transition-colors', onclick: window.closeModal }
        ]
    });

    if (showInvites) {
        loadInvitesList(id);
    }
}

window.updateSettingsUI = function() {
    const vis = document.getElementById('setting-visibility').value;
    const lic = document.getElementById('setting-license').value;
    
    const invitesSection = document.getElementById('settings-invites-section');
    const geoSection = document.getElementById('settings-geo-section');
    
    const showInvites = (vis === 'private' || lic === 'invited_only');
    const showGeo = (lic === 'geo_time');

    if (invitesSection) {
        const wasHidden = invitesSection.style.display === 'none';
        invitesSection.style.display = showInvites ? 'block' : 'none';
        if (showInvites && wasHidden && currentEventIdForSettings) {
            loadInvitesList(currentEventIdForSettings);
        }
    }
    
    if (geoSection) {
        geoSection.style.display = showGeo ? 'block' : 'none';
    }
}

// Alias for compatibility if needed, but updateSettingsUI handles both
window.toggleInvites = window.updateSettingsUI;

window.setEventLocationFromDevice = function() {
    if (!navigator.geolocation) {
        window.showAlert({ title: 'Error', content: 'Geolocation is not supported.' });
        return;
    }
    navigator.geolocation.getCurrentPosition((pos) => {
        document.getElementById('setting-geo-lat').value = pos.coords.latitude;
        document.getElementById('setting-geo-lng').value = pos.coords.longitude;
    }, (err) => {
        window.showAlert({ title: 'Error', content: 'Could not get location: ' + err.message });
    });
}

async function loadInvitesList(eventId) {
    const ul = document.getElementById('settings-invites-list');
    if (!ul) return;

    const res = await authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/invites`);
    if (!res.ok) {
        ul.innerHTML = '<li>Failed to load invites.</li>';
        return;
    }
    const invites = await res.json();
    ul.innerHTML = '';

    if (!invites || invites.length === 0) {
        ul.innerHTML = '<li>No active invites.</li>';
        return;
    }

    // Resolve usernames in parallel
    const resolvedInvites = await Promise.all(invites.map(async (inv) => {
        try {
            const userRes = await authService.fetchWithAuth(authService.apiUrl + `/users/${inv.userId}`);
            if (userRes.ok) {
                const user = await userRes.json();
                return { ...inv, username: user.username, displayName: user.displayName };
            }
        } catch (e) {
            console.error('Failed to resolve user', inv.userId, e);
        }
        return { ...inv, username: inv.userId }; // Fallback to ID
    }));

    resolvedInvites.forEach(inv => {
        const li = document.createElement('li');
        li.className = 'flex justify-between items-center bg-white/5 px-2 py-1 rounded';
        const displayName = inv.displayName ? `${inv.displayName} (@${inv.username})` : inv.username;
        li.innerHTML = `
           <span class="truncate pr-2" title="${inv.userId}">${displayName}</span>
           <button onclick="removeInvite('${inv.userId}')" class="text-error hover:text-red-400">&times;</button>
        `;
        ul.appendChild(li);
    });
}

window.sendInvite = async function() {
    const userIdInput = document.getElementById('invite-user-id');
    let userId = userIdInput.value.trim();
    if (!userId) return;

    // Resolve username to UUID if needed
    const uuidRegex = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;
    if (!uuidRegex.test(userId)) {
        // Assume it's a username or display name, try to resolve it
        try {
            const searchRes = await authService.fetchWithAuth(authService.apiUrl + '/users/search?query=' + encodeURIComponent(userId));
            if (searchRes.ok) {
                const searchData = await searchRes.json();
                if (searchData.items && searchData.items.length > 0) {
                    // Try to find exact match on username
                    const exactMatch = searchData.items.find(u => u.username.toLowerCase() === userId.toLowerCase() || u.displayName.toLowerCase() === userId.toLowerCase());
                    if (exactMatch) {
                        userId = exactMatch.userId;
                    } else {
                        // Use the first result if no exact match
                        userId = searchData.items[0].userId;
                    }
                } else {
                    window.showAlert({ title: 'Error', content: 'User not found.' });
                    return;
                }
            } else {
                window.showAlert({ title: 'Error', content: 'Failed to search for user.' });
                return;
            }
        } catch (e) {
            console.error('User resolution failed', e);
            window.showAlert({ title: 'Error', content: 'Failed to resolve user.' });
            return;
        }
    }

    const res = await authService.fetchWithAuth(authService.apiUrl + `/events/${currentEventIdForSettings}/invites`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ userId })
    });

    if (res.ok) {
        userIdInput.value = '';
        loadInvitesList(currentEventIdForSettings);
        window.showToast('User invited.');

        // Sync invite to playlist
        try {
            const eventName = document.getElementById('ev-name')?.textContent;
            if (eventName && currentEventIdForSettings) {
                // Fetch event details to ensure context
                let eventObj = null;
                const evRes = await authService.fetchWithAuth(authService.apiUrl + '/events/' + currentEventIdForSettings);
                if (evRes.ok) eventObj = await evRes.json();

                const playlistId = await getOrCreateEventPlaylist(currentEventIdForSettings, eventName, eventObj);
                if (playlistId) {
                    await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/invites`, {
                        method: 'POST',
                        headers: { 'content-type': 'application/json' },
                        body: JSON.stringify({ userId })
                    });
                }
            }
        } catch (e) {
            console.error('Failed to sync invite to playlist', e);
        }

    } else {
        const err = await res.json().catch(() => ({}));
        window.showAlert({ title: 'Error', content: err.error || 'Failed to invite user.' });
    }
}

window.removeInvite = async function(userId) {
     const res = await authService.fetchWithAuth(authService.apiUrl + `/events/${currentEventIdForSettings}/invites/${userId}`, {
        method: 'DELETE'
    });
    if (res.ok) {
        loadInvitesList(currentEventIdForSettings);

        // Sync remove invite from playlist
        try {
            const eventName = document.getElementById('ev-name')?.textContent;
            if (eventName && currentEventIdForSettings) {
                // Fetch event details
                let eventObj = null;
                const evRes = await authService.fetchWithAuth(authService.apiUrl + '/events/' + currentEventIdForSettings);
                if (evRes.ok) eventObj = await evRes.json();

                const playlistId = await getOrCreateEventPlaylist(currentEventIdForSettings, eventName, eventObj);
                if (playlistId) {
                    await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/invites/${userId}`, {
                        method: 'DELETE'
                    });
                }
            }
        } catch (e) {
            console.error('Failed to sync remove invite from playlist', e);
        }

    } else {
        window.showAlert({ title: 'Error', content: 'Failed to remove invite.' });
    }
}

window.saveEventSettings = async function() {
    const visibility = document.getElementById('setting-visibility').value;
    const licenseMode = document.getElementById('setting-license').value;
    
    const payload = { visibility, licenseMode };

    if (licenseMode === 'geo_time') {
        const latStr = document.getElementById('setting-geo-lat').value;
        const lngStr = document.getElementById('setting-geo-lng').value;
        const radStr = document.getElementById('setting-geo-radius').value;

        if (latStr && lngStr) {
            payload.geoLat = parseFloat(latStr);
            payload.geoLng = parseFloat(lngStr);
        }
        if (radStr) {
            payload.geoRadiusM = parseInt(radStr, 10);
        }
    }
    
    const res = await authService.fetchWithAuth(authService.apiUrl + '/events/' + currentEventIdForSettings, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(payload)
    });

    if (res.ok) {
        window.closeModal();
        window.showAlert({ title: 'Success', content: 'Settings updated.' });
        loadEvent(currentEventIdForSettings); // Refresh main view
        
        // Sync playlist settings
        try {
            const eventName = document.getElementById('ev-name')?.textContent;
            if (eventName) {
                // Pass new settings via event object simulation to get/create correctly
                const eventObj = { visibility, licenseMode };
                const playlistId = await getOrCreateEventPlaylist(currentEventIdForSettings, eventName, eventObj);
                
                if (playlistId) {
                    let plPublic = true;
                    
                    // Enforce Private Playlist if Event is Private
                    if (visibility === 'private') {
                        plPublic = false;
                    }
                    
                    await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + playlistId, {
                        method: 'PATCH',
                        headers: { 'content-type': 'application/json' },
                        body: JSON.stringify({ isPublic: plPublic, editMode: 'invited' })
                    });

                    // Sync invites to ensure "exact same users invited"
                    const invitesRes = await authService.fetchWithAuth(authService.apiUrl + `/events/${currentEventIdForSettings}/invites`);
                    if (invitesRes.ok) {
                        const invites = await invitesRes.json();
                        if (invites && invites.length > 0) {
                            await Promise.all(invites.map(inv => 
                                authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/invites`, {
                                    method: 'POST',
                                    headers: { 'content-type': 'application/json' },
                                    body: JSON.stringify({ userId: inv.userId })
                                })
                            ));
                        }
                    }
                }
            }
        } catch (e) {
            console.error('Failed to sync playlist settings', e);
        }

    } else {
        const err = await res.json().catch(() => ({}));
        window.showAlert({ title: 'Error', content: err.error || 'Update failed.' });
    }
}

async function loadTally(id) {
    const eventId = id || document.getElementById('ev-id').textContent;
    if (!eventId) return;

    // Fetch Tally and Event Name (if needed for playlist lookup)
    const [tallyRes, eventRes] = await Promise.all([
        authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/tally`),
        authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}`)
    ]);

    if (!tallyRes.ok) return;
    const tally = await tallyRes.json();
    
    // Fetch Playlist Tracks to Filter
    let excludedIds = new Set();
    if (eventRes.ok) {
        const ev = await eventRes.json();
        const playlistId = await getOrCreateEventPlaylist(eventId, ev.name, ev);
        if (playlistId) {
             const plRes = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}`);
             if (plRes.ok) {
                 const plData = await plRes.json();
                 if (plData.tracks) {
                     plData.tracks.forEach(t => {
                         if (t.providerTrackId) excludedIds.add(t.providerTrackId);
                     });
                 }
             }
        }
    }

    const container = document.getElementById('tally-list')
    if (!container) return

    container.innerHTML = ''
    
    // Handle null tally
    const tallyList = tally || [];

    // Filter tally
    const filteredTally = tallyList.filter(row => {
        try {
            const t = row.track.startsWith('{') ? JSON.parse(row.track) : { title: row.track, id: row.track };
            const pid = t.id || t.providerTrackId;
            // If no ID, we show it (can't check against playlist)
            if (!pid) return true;
            return !excludedIds.has(pid);
        } catch (e) { return true; }
    });

    if (!filteredTally || filteredTally.length === 0) {
        container.innerHTML = '<p class="text-text-muted py-4 text-center">No votes yet (or all voted tracks are in playlist).</p>'
        return
    }

    filteredTally.forEach((row, index) => {
        const item = document.createElement('div')
        item.className = 'flex items-center p-3 bg-white/5 rounded-md gap-3'
        
        // Rank
        const rank = document.createElement('div');
        rank.className = 'text-xl font-bold text-primary w-8 text-center flex-shrink-0';
        rank.textContent = (index + 1) + '.';
        item.appendChild(rank);
        
        let displayTitle = row.track;
        let displayArtist = '';

        try {
            if (row.track.startsWith('{')) {
                const data = JSON.parse(row.track);
                displayTitle = data.title || displayTitle;
                displayArtist = data.artist || '';
            }
        } catch (e) {}

        const trackInfo = document.createElement('div')
        trackInfo.className = 'flex flex-col min-w-0 flex-1'
        
        const titleDiv = document.createElement('div')
        titleDiv.className = 'font-medium truncate'
        titleDiv.textContent = decodeHTMLEntities(displayTitle)
        trackInfo.appendChild(titleDiv)

        if (displayArtist) {
            const artistDiv = document.createElement('div')
            artistDiv.className = 'text-xs text-text-muted truncate'
            artistDiv.textContent = decodeHTMLEntities(displayArtist)
            trackInfo.appendChild(artistDiv)
        }
        
        const count = document.createElement('div')
        count.className = 'text-text font-bold ml-2 whitespace-nowrap'
        count.textContent = row.count + (row.count === 1 ? ' vote' : ' votes')
        
        item.appendChild(trackInfo)
        item.appendChild(count)
        container.appendChild(item)
    })
}

// ... existing imports ...

// Ensure mrState exists
window.mrState = window.mrState || {};
window.mrState.eventPlaylistIds = window.mrState.eventPlaylistIds || {};

async function getOrCreateEventPlaylist(eventId, eventName, eventObj) {
    // Check cache first
    if (eventId && window.mrState.eventPlaylistIds[eventId]) {
        return window.mrState.eventPlaylistIds[eventId];
    }

    const res = await authService.fetchWithAuth(authService.apiUrl + '/playlists');
    if (!res.ok) return null;
    const playlists = await res.json();

    // Naming convention for event playlists
    const expectedName = `Event: ${eventName}`; 
    const existing = playlists.find(p => p.name === expectedName || p.name === `Event: ${eventName} (${eventId})`);
    
    if (existing) {
        if (eventId) window.mrState.eventPlaylistIds[eventId] = existing.id;
        return existing.id;
    }

    // Determine settings
    let isPublic = true;
    const editMode = 'invited'; // ALWAYS restricted for event playlists (owner + invited)

    // If event object provided, enforce rules
    if (eventObj) {
        // Rule: If Event is Private => Playlist Private
        if (eventObj.visibility === 'private') {
            isPublic = false;
        }
    }

    // Create if not exists
    const createRes = await authService.fetchWithAuth(authService.apiUrl + '/playlists', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: expectedName, isPublic: isPublic, editMode: editMode })
    });

    if (createRes.ok) {
        const newPl = await createRes.json();
        const playlistId = newPl.id;
        
        if (eventId) window.mrState.eventPlaylistIds[eventId] = playlistId;

        // Sync initial invites from event to playlist
        if (eventId) {
            try {
                const invitesRes = await authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/invites`);
                if (invitesRes.ok) {
                    const invites = await invitesRes.json();
                    if (invites && invites.length > 0) {
                        // Add each invite to the playlist
                        await Promise.all(invites.map(inv => 
                            authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/invites`, {
                                method: 'POST',
                                headers: { 'content-type': 'application/json' },
                                body: JSON.stringify({ userId: inv.userId })
                            })
                        ));
                    }
                }
            } catch (e) {
                console.error('Failed to sync initial invites to playlist', e);
            }
        }

        return playlistId;
    }
    return null;
}

async function syncEventPlaylist(eventId, eventName) {
    // Attempt to fetch event for context
    let eventObj = null;
    try {
        const evRes = await authService.fetchWithAuth(authService.apiUrl + '/events/' + eventId);
        if (evRes.ok) eventObj = await evRes.json();
    } catch (e) {}

    const playlistId = await getOrCreateEventPlaylist(eventId, eventName, eventObj);
    if (!playlistId) return;

    // Get Tally (sorted by votes)
    const tallyRes = await authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/tally`);
    if (!tallyRes.ok) return;
    const tally = await tallyRes.json();

    // Get Playlist Tracks
    const plRes = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}`);
    if (!plRes.ok) return;
    const plData = await plRes.json();
    // Refresh local track list as we modify it
    let currentTracks = plData.tracks || [];

    // Sync
    for (let i = 0; i < tally.length; i++) {
        const voteItem = tally[i];
        let voteTrack = {};
        try {
             if (voteItem.track.startsWith('{')) {
                voteTrack = JSON.parse(voteItem.track);
             } else {
                voteTrack = { title: voteItem.track };
             }
        } catch (e) { continue; }
        
        // We need an ID (provider ID) to add it.
        // If the vote was just a raw string (legacy), we might skip or search it? 
        // For now, assume new system with JSON payload.
        const providerId = voteTrack.id || voteTrack.providerTrackId;
        if (!providerId) continue; 

        // Check if in playlist
        const existingTrack = currentTracks.find(t => 
            (t.providerTrackId && t.providerTrackId === providerId)
        );

        if (!existingTrack) {
            // Add it
             await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/tracks`, {
                method: 'POST',
                headers: { 'content-type': 'application/json' },
                body: JSON.stringify({
                    title: voteTrack.title,
                    artist: voteTrack.artist,
                    provider: voteTrack.provider || 'youtube',
                    providerTrackId: providerId,
                    thumbnailUrl: voteTrack.thumbnailUrl
                })
            });
            // Don't re-fetch immediately for performance, just assume added at end? 
            // Actually, we need to know its ID to move it later if needed.
            // So we should re-fetch playlist tracks or response.
            const updatedPlRes = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}`);
            if (updatedPlRes.ok) {
                const updatedData = await updatedPlRes.json();
                currentTracks = updatedData.tracks || [];
            }
        } else {
            // Check position (index i)
            const currentIdx = currentTracks.indexOf(existingTrack);
            if (currentIdx !== i) {
                // Move
                 await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/tracks/${existingTrack.id}`, {
                    method: 'PATCH',
                    headers: { 'content-type': 'application/json' },
                    body: JSON.stringify({ newPosition: i })
                });
                // Update local list to avoid confusion? 
                // A move changes other indices. Re-fetching is safest.
                const updatedPlRes = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}`);
                if (updatedPlRes.ok) {
                    const updatedData = await updatedPlRes.json();
                    currentTracks = updatedData.tracks || [];
                }
            }
        }
    }
}

window.castVote = async function(trackIdPayload) {
    const eventId = document.getElementById('ev-id').textContent;
    const eventName = document.getElementById('ev-name').textContent;
    let payload = { trackId: trackIdPayload };

    if (currentEventLicenseMode === 'geo_time') {
        if (!navigator.geolocation) {
             window.showAlert({ title: 'Error', content: 'Geolocation is not supported by your browser.' });
             return;
        }
        
        try {
            const position = await new Promise((resolve, reject) => {
                navigator.geolocation.getCurrentPosition(resolve, reject);
            });
            payload.lat = position.coords.latitude;
            payload.lng = position.coords.longitude;
        } catch (e) {
             window.showAlert({ title: 'Error', content: 'Location access is required to vote in this event. Please allow location access.' });
             return;
        }
    }

    const res = await authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/vote`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(payload)
    });

    if (res.ok) {
        window.showAlert({ title: 'Success', content: 'Vote cast!' });
        await loadTally(eventId);
        // Sync playlist removed (wait for round end)
        // syncEventPlaylist(eventId, eventName);
    } else if (res.status === 409) {
        window.showAlert({ title: 'Vote Recorded', content: 'You have already voted for a track in this event.' });
    } else {
        const err = await res.json().catch(() => ({}));
        window.showAlert({ title: 'Error', content: 'Failed to vote: ' + (err.error || 'Unknown error') });
    }
}

// --- Voting Round Timer ---

// We store the interval ID in the global mrState to persist across HTMX swaps/re-executions
window.mrState = window.mrState || {};

window.startVotingRound = function(passedEventId) {
    // Get current event ID (from arg or DOM)
    const eventId = passedEventId || document.getElementById('ev-id')?.textContent;
    if (!eventId) return;

    // 1 minute default
    const durationSec = 60;
    const now = Date.now();
    const endTime = now + (durationSec * 1000);

    // Persist to LocalStorage
    localStorage.setItem('mr_round_end_' + eventId, endTime);
    
    startTimerInterval(eventId, endTime);
    
    // Only toast if on page
    if (document.getElementById('ev-id')?.textContent === eventId) {
        window.showToast('Voting round started! 1 minute remaining.');
    }
}

function startTimerInterval(eventId, endTime) {
    // Clear any existing interval for this event or generally
    if (window.mrState['roundInterval_' + eventId]) {
        clearInterval(window.mrState['roundInterval_' + eventId]);
    }

    // Update UI immediately
    updateTimerUI(true, eventId);

    const intervalId = setInterval(() => {
        const now = Date.now();
        const remaining = Math.ceil((endTime - now) / 1000);

        const timerVal = document.getElementById('timer-val');
        // Only update UI if we are on the correct event page
        const currentPageId = document.getElementById('ev-id')?.textContent;
        
        if (currentPageId === eventId && timerVal) {
            timerVal.textContent = remaining > 0 ? remaining + 's' : '0s';
        }

        if (remaining <= 0) {
            endVotingRound(eventId);
        }
    }, 1000);

    window.mrState['roundInterval_' + eventId] = intervalId;
    
    // Initial UI update for text
    const timerVal = document.getElementById('timer-val');
    if (timerVal) {
        const remaining = Math.ceil((endTime - Date.now()) / 1000);
        timerVal.textContent = remaining > 0 ? remaining + 's' : '0s';
    }
}

function updateTimerUI(isActive, eventId) {
    const currentPageId = document.getElementById('ev-id')?.textContent;
    if (currentPageId !== eventId) return;

    const timerDisplay = document.getElementById('round-timer-display');
    const startBtn = document.getElementById('btn-start-round');
    
    if (isActive) {
        if (startBtn) startBtn.style.display = 'none';
        if (timerDisplay) timerDisplay.style.display = 'block';
    } else {
        if (timerDisplay) timerDisplay.style.display = 'none';
        if (startBtn) startBtn.style.display = 'inline-block';
    }
}

async function endVotingRound(eventId) {
    // Cleanup
    if (window.mrState['roundInterval_' + eventId]) {
        clearInterval(window.mrState['roundInterval_' + eventId]);
        delete window.mrState['roundInterval_' + eventId];
    }
    localStorage.removeItem('mr_round_end_' + eventId);
    
    updateTimerUI(false, eventId);

    const onPage = (document.getElementById('ev-id')?.textContent === eventId);
    if (onPage) window.showToast('Round ended! Processing winner...');

    let shouldRestart = true;
    let isOwner = false;

    try {
        // 1. Get Event Details
        let eventName = document.getElementById('ev-name')?.textContent;
        let eventObj = null;

        // Always fetch event to ensure we have latest settings for playlist creation
        const evRes = await authService.fetchWithAuth(authService.apiUrl + '/events/' + eventId);
        if (evRes.status === 404) {
            console.log('Event not found, stopping timer loop and cleaning up localStorage.');
            localStorage.removeItem('mr_round_end_' + eventId);
            shouldRestart = false;
            return;
        }
        if (evRes.ok) {
            eventObj = await evRes.json();
            eventName = eventObj.name;
        } else if (!eventName) {
            eventName = 'Event';
        }

        // Check ownership
        try {
            const meRes = await authService.fetchWithAuth(authService.apiUrl + '/users/me');
            if (meRes.ok && eventObj) {
                const me = await meRes.json();
                if (me.userId === eventObj.ownerId) {
                    isOwner = true;
                }
            }
        } catch (e) {
            console.error('Failed to verify ownership', e);
        }
        
        console.log('endVotingRound: isOwner =', isOwner);

        // 2. Get Playlist & Tracks (to filter out winners)
        const playlistId = await getOrCreateEventPlaylist(eventId, eventName, eventObj);
        let existingTrackIds = new Set();
        
        if (playlistId) {
            const plRes = await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + playlistId);
            if (plRes.ok) {
                 const plData = await plRes.json();
                 if (plData.tracks) {
                     plData.tracks.forEach(t => {
                         if (t.providerTrackId) existingTrackIds.add(t.providerTrackId);
                     });
                 }
            }
        }

        // 3. Get Tally
        const tallyRes = await authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/tally`);
        if (!tallyRes.ok) {
            if(onPage) window.showAlert({ title: 'Error', content: 'Failed to fetch tally.' });
            return; 
        }
        let tally = await tallyRes.json();
        if (!tally) tally = [];
        
        // 4. Filter Tally
        const candidates = tally.filter(row => {
            let trackObj = {};
            try {
                trackObj = row.track.startsWith('{') ? JSON.parse(row.track) : { title: row.track, id: row.track };
            } catch (e) { return false; }
            
            const pid = trackObj.id || trackObj.providerTrackId;
            // If no ID, we can't filter by existence in playlist easily, but we'll allow it for now
            return !pid || !existingTrackIds.has(pid);
        });

        if (candidates.length === 0) {
            if(onPage) window.showToast('No new votes (or all candidates already in playlist). Round skipped.');
            return;
        }

        // 5. Identify Winner
        const winner = candidates[0]; 
        let winnerTrack = {};
        try {
            winnerTrack = winner.track.startsWith('{') ? JSON.parse(winner.track) : { title: winner.track, id: winner.track };
        } catch (e) {
            winnerTrack = { title: winner.track, id: winner.track };
        }
        
        const providerId = winnerTrack.id || winnerTrack.providerTrackId;

        // 6. Add to Playlist (Only Owner)
        if (isOwner && playlistId && providerId) {
             console.log('Adding winner to playlist:', winnerTrack.title);
             const addRes = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/tracks`, {
                method: 'POST',
                headers: { 'content-type': 'application/json' },
                body: JSON.stringify({
                    title: winnerTrack.title,
                    artist: winnerTrack.artist || 'Unknown',
                    provider: winnerTrack.provider || 'youtube',
                    providerTrackId: providerId,
                    thumbnailUrl: winnerTrack.thumbnailUrl || ''
                })
            });
            
            if (addRes.ok) {
                 if(onPage) window.showToast(`Added "${winnerTrack.title}" to playlist.`);
                 else console.log(`Added "${winnerTrack.title}" to playlist.`);
            } else {
                 const errTxt = await addRes.text();
                 console.error('Failed to add to playlist:', addRes.status, errTxt);
            }
        }
        
        // 7. Clear Votes for Winner Only (Carry-over logic) - Only Owner
        if (isOwner) {
            // We pass the track string as it is stored in the DB
            const delRes = await authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/votes?track=` + encodeURIComponent(winner.track), {
                method: 'DELETE'
            });
            
            if (!delRes.ok) {
                console.warn('Failed to clear votes for next round');
            }
        }

        // Everyone refreshes tally to see the winner removed
        if (onPage) loadTally(eventId);

        if(onPage) window.showToast('Round ended. Winner processed.');

    } catch (e) {
        console.error('Error in endVotingRound:', e);
    } finally {
        // Auto-restart next round - ONLY OWNER should start the next round to avoid chaos
        if (shouldRestart && isOwner) {
            setTimeout(() => startVotingRound(eventId), 2000);
        }
    }
}

// Check for active round on load
function checkActiveRound() {
    const eventId = document.getElementById('ev-id')?.textContent;
    if (!eventId) return;

    const storedEnd = localStorage.getItem('mr_round_end_' + eventId);
    if (storedEnd) {
        const endTime = parseInt(storedEnd, 10);
        const now = Date.now();
        if (endTime > now) {
            // Resume
            console.log('Resuming active round timer for event', eventId);
            startTimerInterval(eventId, endTime);
        } else {
            // Expired while away? Trigger end logic immediately
            console.log('Round expired while away, ending now.');
            endVotingRound(eventId);
        }
    }
}

window.openCreateEventModal = function() {
  window.showPrompt({
    title: 'Create a new event',
    content: 'Enter a name for your new event:',
    onConfirm: async (name) => {
      if (!name || name.length < 3) {
        window.showAlert({ title: 'Warning', content: 'Event name must be at least 3 characters long.' })
        return
      }

      const res = await authService.fetchWithAuth(authService.apiUrl + '/events', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: name, visibility: 'public', licenseMode: 'everyone' })
      })

      if (res.ok) {
        const newEvent = await res.json().catch(() => null);
        
        // Auto-create playlist
        if (newEvent && newEvent.id) {
            getOrCreateEventPlaylist(newEvent.id, newEvent.name, newEvent);
        }

        window.showAlert({ title: 'Success', content: 'Event created successfully!' })
        refreshEvents();
      } else {
        const errorJson = await res.json().catch(() => ({}))
        window.showAlert({ title: 'Error', content: (errorJson && errorJson.error) || 'Unknown error' })
      }
    }
  })
}


function renameEvent(id, currentName) {
  window.showPrompt({
    title: 'Rename Event',
    content: 'Enter new name:',
    onConfirm: async (newName) => {
      if (!newName || newName.length < 3) {
        window.showAlert({ title: 'Warning', content: 'Name must be at least 3 characters long.' })
        return
      }
      const res = await authService.fetchWithAuth(authService.apiUrl + '/events/' + id, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: newName })
      })
      if (res.ok) {
         await refreshEvents();
         
         // Rename Playlist
         try {
             // Use OLD name to find it
             const playlistId = await getOrCreateEventPlaylist(id, currentName, null);
             if (playlistId) {
                 const newPlaylistName = `Event: ${newName}`;
                 await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + playlistId, {
                    method: 'PATCH',
                    headers: { 'content-type': 'application/json' },
                    body: JSON.stringify({ name: newPlaylistName })
                 });
             }
         } catch (e) {
             console.error('Failed to rename associated playlist', e);
         }

      } else {
         const err = await res.json().catch(() => ({}))
         window.showAlert({ title: 'Error', content: (err && err.error) || 'Unknown error' })
      }
    }
  })
}

async function deleteEvent(id) {
    // 1. Get Event Name to find playlist
    // We might not be on the event page, so we need to try to get the name first.
    // However, if we delete the event first, we might lose the name.
    // So let's try to find the playlist BEFORE deleting the event.
    
    // We can't easily get the name if we are on the list page without fetching or parsing DOM.
    // If we are on list page, the DOM element with data-id has the name.
    let eventName = null;
    const listEl = document.querySelector(`div[data-id="${id}"] .font-bold`);
    if (listEl) {
        eventName = listEl.textContent;
    } else {
        // Maybe on detail page?
        const detailEl = document.getElementById('ev-name');
        const detailId = document.getElementById('ev-id');
        if (detailEl && detailId && detailId.textContent === id) {
            eventName = detailEl.textContent;
        }
    }

    // Fallback: fetch event if name not found
    if (!eventName) {
        try {
            const evRes = await authService.fetchWithAuth(authService.apiUrl + '/events/' + id);
            if (evRes.ok) {
                const ev = await evRes.json();
                eventName = ev.name;
            }
        } catch (e) {}
    }

    window.showConfirm({
        title: 'Delete Event',
        content: 'Are you sure you want to delete this event? This will also delete the associated playlist.',
        onConfirm: async () => {
            // Delete Event
            const res = await authService.fetchWithAuth(authService.apiUrl + '/events/' + id, {
                method: 'DELETE'
            })

            if (res.ok) {
                await refreshEvents();
                window.showAlert({ title: 'Success', content: 'Event deleted.' });
                
                // Delete Playlist
                if (eventName) {
                    try {
                        // Pass null as eventObj since event is gone, but logic handles name-based lookup
                        const playlistId = await getOrCreateEventPlaylist(id, eventName, null);
                        if (playlistId) {
                            await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + playlistId, {
                                method: 'DELETE'
                            });
                        }
                    } catch (e) {
                        console.error('Failed to delete associated playlist', e);
                    }
                }
            } else {
                const err = await res.json().catch(() => ({}))
                window.showAlert({ title: 'Error', content: (err && err.error) || 'Unknown error' })
            }
        }
    })
}

function decodeHTMLEntities(text) {
    if (!text) return '';
    const textArea = document.createElement('textarea');
    textArea.innerHTML = text;
    return textArea.value;
}

// Global Event Supervisor
window.initGlobalEventSupervisor = function() {
    // Prevent multiple intervals
    if (window.mrState.supervisorInterval) return;
    
    // Track rounds we've already "ended" in this session to prevent loops for non-owners
    window.mrState.processedExpiredRounds = window.mrState.processedExpiredRounds || new Set();

    window.mrState.supervisorInterval = setInterval(() => {
        const now = Date.now();
        // Scan LocalStorage for active rounds
        for (let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            if (key && key.startsWith('mr_round_end_')) {
                const eventId = key.replace('mr_round_end_', '');
                
                // Skip if already processed in this session
                if (window.mrState.processedExpiredRounds.has(eventId)) continue;

                const endTime = parseInt(localStorage.getItem(key), 10);
                
                if (endTime && endTime < now) {
                    // Expired
                    console.log('Global Supervisor: Found expired round for event', eventId);
                    endVotingRound(eventId);
                }
            }
        }
    }, 1000);
}

// ... (previous code) ...

async function endVotingRound(eventId) {
    // Mark as processed in this session so Supervisor doesn't loop
    window.mrState.processedExpiredRounds = window.mrState.processedExpiredRounds || new Set();
    window.mrState.processedExpiredRounds.add(eventId);

    // Cleanup local interval
    if (window.mrState['roundInterval_' + eventId]) {
        clearInterval(window.mrState['roundInterval_' + eventId]);
        delete window.mrState['roundInterval_' + eventId];
    }
    // NOTE: We do NOT remove localStorage key yet. Only Owner can do that after processing.
    
    updateTimerUI(false, eventId);

    const onPage = (document.getElementById('ev-id')?.textContent === eventId);
    if (onPage) window.showToast('Round ended! Checking results...');

    let shouldRestart = true;
    let isOwner = false;

    try {
        // ... (fetch event logic) ...
        // 1. Get Event Details
        let eventName = document.getElementById('ev-name')?.textContent;
        let eventObj = null;

        const evRes = await authService.fetchWithAuth(authService.apiUrl + '/events/' + eventId);
        if (evRes.status === 404) {
            console.log('Event not found, cleaning up.');
            localStorage.removeItem('mr_round_end_' + eventId);
            shouldRestart = false;
            return;
        }
        if (evRes.ok) {
            eventObj = await evRes.json();
            eventName = eventObj.name;
        } else if (!eventName) {
            eventName = 'Event';
        }

        // Check ownership
        try {
            const meRes = await authService.fetchWithAuth(authService.apiUrl + '/users/me');
            if (meRes.ok && eventObj) {
                const me = await meRes.json();
                if (me.userId === eventObj.ownerId) {
                    isOwner = true;
                }
            }
        } catch (e) {
            console.error('Failed to verify ownership', e);
        }
        
        console.log('endVotingRound: isOwner =', isOwner);

        // If NOT owner, we stop here. We've stopped the UI timer.
        // We leave the localStorage key so the Owner can process it when they log in.
        if (!isOwner) {
            if (onPage) window.showToast('Round ended. Waiting for host to finalize results...');
            return;
        }

        // --- OWNER LOGIC BELOW ---

        // 2. Get Playlist & Tracks
        const playlistId = await getOrCreateEventPlaylist(eventId, eventName, eventObj);
        let existingTrackIds = new Set();
        
        if (playlistId) {
            const plRes = await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + playlistId);
            if (plRes.ok) {
                 const plData = await plRes.json();
                 if (plData.tracks) {
                     plData.tracks.forEach(t => {
                         if (t.providerTrackId) existingTrackIds.add(t.providerTrackId);
                     });
                 }
            }
        }

        // 3. Get Tally
        const tallyRes = await authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/tally`);
        if (!tallyRes.ok) {
            if(onPage) window.showToast('Failed to fetch tally. Retrying...');
            // If fetch failed, we might want to retry? For now, we don't clear key so it will retry on refresh.
            // But we already added to processedExpiredRounds, so it won't retry this session.
            // That's acceptable failure mode.
            return; 
        }
        let tally = await tallyRes.json();
        if (!tally) tally = [];
        
        // 4. Filter Tally
        const candidates = tally.filter(row => {
            let trackObj = {};
            try {
                trackObj = row.track.startsWith('{') ? JSON.parse(row.track) : { title: row.track, id: row.track };
            } catch (e) { return false; }
            
            const pid = trackObj.id || trackObj.providerTrackId;
            return !pid || !existingTrackIds.has(pid);
        });

        if (candidates.length === 0) {
            if(onPage) window.showToast('No new votes (or all candidates already in playlist). Round skipped.');
            // Even if skipped, we consider the round "Done".
            localStorage.removeItem('mr_round_end_' + eventId);
            // Restart if owner
            if (shouldRestart) setTimeout(() => startVotingRound(eventId), 2000);
            return;
        }

        // 5. Identify Winner
        const winner = candidates[0]; 
        let winnerTrack = {};
        try {
            winnerTrack = winner.track.startsWith('{') ? JSON.parse(winner.track) : { title: winner.track, id: winner.track };
        } catch (e) {
            winnerTrack = { title: winner.track, id: winner.track };
        }
        
        const providerId = winnerTrack.id || winnerTrack.providerTrackId;

        // 6. Add to Playlist
        if (playlistId && providerId) {
             console.log('Adding winner to playlist:', winnerTrack.title);
             const addRes = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/tracks`, {
                method: 'POST',
                headers: { 'content-type': 'application/json' },
                body: JSON.stringify({
                    title: winnerTrack.title,
                    artist: winnerTrack.artist || 'Unknown',
                    provider: winnerTrack.provider || 'youtube',
                    providerTrackId: providerId,
                    thumbnailUrl: winnerTrack.thumbnailUrl || ''
                })
            });
            
            if (addRes.ok) {
                 if(onPage) window.showToast(`Added "${winnerTrack.title}" to playlist.`);
            } else {
                 console.error('Failed to add to playlist:', addRes.status);
            }
        }
        
        // 7. Clear Votes
        const delRes = await authService.fetchWithAuth(authService.apiUrl + `/events/${eventId}/votes?track=` + encodeURIComponent(winner.track), {
            method: 'DELETE'
        });
        
        if (!delRes.ok) {
            console.warn('Failed to clear votes');
        }

        // Refresh Tally
        if (onPage) loadTally(eventId);

        // FINAL SUCCESS: Remove the timer key
        localStorage.removeItem('mr_round_end_' + eventId);
        
        if(onPage) window.showToast('Round finalized.');

        // Restart
        if (shouldRestart) {
            setTimeout(() => startVotingRound(eventId), 2000);
        }

    } catch (e) {
        console.error('Error in endVotingRound:', e);
    }
}
// Init function to be called after authService is ready
window.initEvents = function() {
    console.log('initEvents called');
    refreshEvents();
    if (window.initGlobalEventSupervisor) {
        window.initGlobalEventSupervisor();
    }
}

// Self-invoke if we are on the events page
if (document.getElementById('events-list')) {
    window.initEvents();
}
