var API = "{{.API}}";

// Avoid re-declaration error with HTMX
if (typeof currentEventIdForSettings === 'undefined') {
    var currentEventIdForSettings = null;
}

async function refreshEvents() {
  const container = document.getElementById('events-list')
  if (!container) return

  const [eventsRes, meRes] = await Promise.all([
      authService.fetchWithAuth(API + '/events'),
      authService.fetchWithAuth(API + '/users/me')
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

    const meta = document.createElement('div')
    meta.className = 'text-sm text-text-muted truncate'
    meta.textContent = `${ev.visibility} • ${ev.licenseMode}`
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
}

async function loadEvent(id) {
    const res = await authService.fetchWithAuth(API + '/events/' + id)
    if (!res.ok) {
        window.showAlert({ title: 'Error', content: 'Failed to load event.' })
        return
    }
    const ev = await res.json()
    document.getElementById('ev-name').textContent = decodeHTMLEntities(ev.name)
    document.getElementById('ev-meta').textContent = `${ev.visibility} • ${ev.licenseMode}`

    // Check ownership
    const meRes = await authService.fetchWithAuth(API + '/users/me');
    if (meRes.ok) {
        const me = await meRes.json();
        if (me.userId === ev.ownerId) {
            const btn = document.getElementById('ev-settings-btn');
            if (btn) btn.classList.remove('hidden');
        }
    }
}

window.openEventSettings = async function() {
    const id = document.getElementById('ev-id').textContent;
    currentEventIdForSettings = id;

    const res = await authService.fetchWithAuth(API + '/events/' + id);
    const ev = await res.json();

    const isPrivate = ev.visibility === 'private';
    const invitesDisplay = isPrivate ? 'block' : 'none';

    // Create Modal Content
    const content = `
      <div class="space-y-4 text-left">
        <div>
           <label class="block text-sm font-medium text-text-muted mb-1">Visibility</label>
           <select id="setting-visibility" onchange="toggleInvites(this.value)" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-text">
             <option value="public" ${!isPrivate ? 'selected' : ''}>Public</option>
             <option value="private" ${isPrivate ? 'selected' : ''}>Private</option>
           </select>
        </div>
        
        <div id="settings-invites-section" class="border-t border-input-border pt-4" style="display: ${invitesDisplay};">
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

    if (isPrivate) {
        loadInvitesList(id);
    }
}

window.toggleInvites = function(val) {
    const el = document.getElementById('settings-invites-section');
    if (el) {
        el.style.display = (val === 'private') ? 'block' : 'none';
        if (val === 'private') {
            loadInvitesList(currentEventIdForSettings);
        }
    }
}

async function loadInvitesList(eventId) {
    const ul = document.getElementById('settings-invites-list');
    if (!ul) return;

    const res = await authService.fetchWithAuth(API + `/events/${eventId}/invites`);
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

    invites.forEach(inv => {
        const li = document.createElement('li');
        li.className = 'flex justify-between items-center bg-white/5 px-2 py-1 rounded';
        li.innerHTML = `
           <span class="truncate pr-2">${inv.userId}</span>
           <button onclick="removeInvite('${inv.userId}')" class="text-error hover:text-red-400">&times;</button>
        `;
        ul.appendChild(li);
    });
}

window.sendInvite = async function() {
    const userIdInput = document.getElementById('invite-user-id');
    const userId = userIdInput.value.trim();
    if (!userId) return;

    const res = await authService.fetchWithAuth(API + `/events/${currentEventIdForSettings}/invites`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ userId })
    });

    if (res.ok) {
        userIdInput.value = '';
        loadInvitesList(currentEventIdForSettings);
        window.showToast('User invited.');
    } else {
        const err = await res.json().catch(() => ({}));
        window.showAlert({ title: 'Error', content: err.error || 'Failed to invite user.' });
    }
}

window.removeInvite = async function(userId) {
     const res = await authService.fetchWithAuth(API + `/events/${currentEventIdForSettings}/invites/${userId}`, {
        method: 'DELETE'
    });
    if (res.ok) {
        loadInvitesList(currentEventIdForSettings);
    } else {
        window.showAlert({ title: 'Error', content: 'Failed to remove invite.' });
    }
}

window.saveEventSettings = async function() {
    const visibility = document.getElementById('setting-visibility').value;
    
    const res = await authService.fetchWithAuth(API + '/events/' + currentEventIdForSettings, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ visibility })
    });

    if (res.ok) {
        window.closeModal();
        window.showAlert({ title: 'Success', content: 'Settings updated.' });
        loadEvent(currentEventIdForSettings); // Refresh main view
    } else {
        const err = await res.json().catch(() => ({}));
        window.showAlert({ title: 'Error', content: err.error || 'Update failed.' });
    }
}

async function loadTally(id) {
    const eventId = id || document.getElementById('ev-id').textContent;
    if (!eventId) return;

    const res = await authService.fetchWithAuth(API + `/events/${eventId}/tally`)
    if (!res.ok) {
        return
    }
    const tally = await res.json()
    const container = document.getElementById('tally-list')
    if (!container) return

    container.innerHTML = ''
    if (!tally || tally.length === 0) {
        container.innerHTML = '<p class="text-text-muted py-4 text-center">No votes yet.</p>'
        return
    }

    tally.forEach(row => {
        const item = document.createElement('div')
        item.className = 'flex justify-between items-center p-3 bg-white/5 rounded-md'
        
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
        trackInfo.className = 'flex flex-col min-w-0 pr-2'
        
        const titleDiv = document.createElement('div')
        titleDiv.className = 'font-medium truncate'
        titleDiv.textContent = displayTitle
        trackInfo.appendChild(titleDiv)

        if (displayArtist) {
            const artistDiv = document.createElement('div')
            artistDiv.className = 'text-xs text-text-muted truncate'
            artistDiv.textContent = displayArtist
            trackInfo.appendChild(artistDiv)
        }
        
        const count = document.createElement('div')
        count.className = 'text-primary font-bold ml-4 whitespace-nowrap'
        count.textContent = row.count + (row.count === 1 ? ' vote' : ' votes')
        
        item.appendChild(trackInfo)
        item.appendChild(count)
        container.appendChild(item)
    })
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

      const res = await authService.fetchWithAuth(API + '/events', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: name, visibility: 'public', licenseMode: 'everyone' })
      })

      if (res.ok) {
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
      const res = await authService.fetchWithAuth(API + '/events/' + id, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: newName })
      })
      if (res.ok) {
         await refreshEvents()
      } else {
         const err = await res.json().catch(() => ({}))
         window.showAlert({ title: 'Error', content: (err && err.error) || 'Unknown error' })
      }
    }
  })
}

async function deleteEvent(id) {
    window.showConfirm({
        title: 'Delete Event',
        content: 'Are you sure you want to delete this event?',
        onConfirm: async () => {
            const res = await authService.fetchWithAuth(API + '/events/' + id, {
                method: 'DELETE'
            })

            if (res.ok) {
                await refreshEvents()
                window.showAlert({ title: 'Success', content: 'Event deleted.' })
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

// Initial load
refreshEvents();