var WS_URL = "{{.WS}}";

async function loadPlaylist(playlistId) {
  const res = await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + playlistId)
  if (!res.ok) {
    window.showAlert({ title: 'Error', content: 'Failed to load playlist.' })
    return
  }
  const data = await res.json() // The response is PlaylistWithTracks
  const playlist = data.playlist;
  const tracks = data.tracks;
  const canEdit = data.canEdit;

  const currentPlaylistDiv = document.getElementById('current-playlist')
  if (currentPlaylistDiv) currentPlaylistDiv.style.display = 'block'
  
  // Show/Hide Search section based on canEdit
  const searchSection = document.getElementById('music-search-query')?.closest('.card');
  if (searchSection) {
      searchSection.style.display = canEdit ? 'block' : 'none';
  }

  const plName = document.getElementById('pl-name')
  if (plName) plName.textContent = decodeHTMLEntities(playlist.name)
  
  const plMeta = document.getElementById('pl-meta')
  if (plMeta) {
      const vis = playlist.isPublic ? 'Public' : 'Private';
      const edit = playlist.editMode === 'invited' ? 'Restricted Edit' : 'Open Edit';
      plMeta.textContent = `${vis} • ${edit} • ${decodeHTMLEntities(playlist.description || '')}`;
  }

  const plId = document.getElementById('pl-id')
  if (plId) plId.textContent = playlist.id

  // Check ownership to show settings
  const meRes = await authService.fetchWithAuth(authService.apiUrl + '/users/me');
  if (meRes.ok) {
      const me = await meRes.json();
      if (me.userId === playlist.ownerId) {
          const btn = document.getElementById('pl-settings-btn');
          if (btn) btn.classList.remove('hidden');
      }
  }

  const tracksUl = document.getElementById('pl-tracks')
  if (tracksUl) {
      tracksUl.innerHTML = ''
      if (tracks && tracks.length > 0) {
        tracks.forEach((track, index) => {
          const li = document.createElement('li')
          li.className = 'flex justify-between items-center py-3 px-4 bg-white/5 rounded-md hover:bg-white/10 transition-colors mb-2'
          
          // Drag and Drop (Only if canEdit is true)
          if (track.id && canEdit) {
              li.setAttribute('draggable', 'true')
              li.dataset.index = index
              li.dataset.trackId = track.id
              li.ondragstart = handleDragStart
              li.ondragover = handleDragOver
              li.ondragend = handleDragEnd
              li.ondrop = function(e) { handleDrop.call(this, e, playlistId) }
          } else {
              li.setAttribute('draggable', 'false')
              li.classList.add('cursor-default')
          }
          
          const infoDiv = document.createElement('div')
          infoDiv.className = 'flex items-center gap-4 flex-1 min-w-0'
          
          // ... (Rest of play button logic)
          if (track.providerTrackId) {
              const btnPlay = document.createElement('button')
              btnPlay.className = 'text-primary hover:text-primary-hover transition-colors flex-shrink-0'
              btnPlay.innerHTML = `
                <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              `
              // Use setQueue to enable Next/Prev functionality
              btnPlay.onclick = () => setQueue(tracks, index, playlistId)
              infoDiv.appendChild(btnPlay)
          } else {
              // Placeholder for old tracks without ID
              const placeholder = document.createElement('div')
              placeholder.className = 'h-10 w-10 flex items-center justify-center text-text-muted opacity-20'
              placeholder.innerHTML = `
                <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                   <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
                </svg>
              `
              placeholder.title = 'Track not playable (missing ID)'
              infoDiv.appendChild(placeholder)
          }

          const textDiv = document.createElement('div')
          textDiv.className = 'truncate'
          
          // Clean title: remove artist prefix if present
          let displayTitle = decodeHTMLEntities(track.title);
          let artist = decodeHTMLEntities(track.artist);
          if (artist && displayTitle.toLowerCase().startsWith(artist.toLowerCase())) {
              displayTitle = displayTitle.substring(artist.length).replace(/^[\s\-\—\:]+/, '');
          }

          const titleSpan = document.createElement('div')
          titleSpan.className = 'font-bold text-lg truncate'
          titleSpan.textContent = displayTitle
          
          const artistSpan = document.createElement('div')
          artistSpan.className = 'text-sm text-text-muted truncate'
          artistSpan.textContent = artist
          
          textDiv.appendChild(titleSpan)
          textDiv.appendChild(artistSpan)
          infoDiv.appendChild(textDiv)
          
          li.appendChild(infoDiv)

          // Only show delete button if canEdit is true
          if (canEdit) {
              const btnDelete = document.createElement('button')
              btnDelete.className = 'inline-flex items-center justify-center text-primary hover:text-primary-hover transition-colors p-2'
              btnDelete.title = 'Remove Track'
              btnDelete.innerHTML = `
                <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              `
              btnDelete.onclick = () => deleteTrack(playlistId, track.id)
              li.appendChild(btnDelete)
          }

          tracksUl.appendChild(li)
        })
      } else {
        const li = document.createElement('li')
        li.className = 'py-4 text-center text-text-muted'
        li.textContent = 'This playlist is empty.'
        tracksUl.appendChild(li)
      }
  }
  
  if (typeof connectPlaylistWS === 'function') {
      connectPlaylistWS(playlistId);
  }
}

async function deleteTrack(playlistId, trackId) {
    window.showConfirm({
        title: 'Remove Track',
        content: 'Are you sure you want to remove this track?',
        onConfirm: async () => {
            const res = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/tracks/${trackId}`, {
                method: 'DELETE'
            })

            if (res.ok) {
                window.showAlert({ title: 'Success', content: 'Track removed.' })
                loadPlaylist(playlistId)
            } else {
                const err = await res.json().catch(() => ({}))
                window.showAlert({ title: 'Error', content: 'Failed to remove track: ' + (err.error || 'Unknown error') })
            }
        }
    })
}

async function searchMusic() {
  const query = document.getElementById('music-search-query').value
  if (!query) return

  const res = await authService.fetchWithAuth(authService.apiUrl + '/music/search?query=' + encodeURIComponent(query))
  const json = await res.json()

  const ul = document.getElementById('music-search-results')
  ul.innerHTML = ''

  if (json.items && json.items.length > 0) {
    json.items.forEach(item => {
      // Decode entities for internal data and display
      item.title = decodeHTMLEntities(item.title);
      item.artist = decodeHTMLEntities(item.artist);

      const li = document.createElement('li')
      li.className = 'flex justify-between items-center py-2 px-3 hover:bg-white/5 rounded transition-colors'

      // Clean search title
      let displayTitle = item.title;
      if (item.artist && displayTitle.toLowerCase().startsWith(item.artist.toLowerCase())) {
          displayTitle = displayTitle.substring(item.artist.length).replace(/^[\s\-\—\:]+/, '');
      }

      const span = document.createElement('span');
      span.className = 'truncate pr-4'
      span.innerHTML = `<span class="font-medium">${displayTitle}</span> <span class="text-xs text-text-muted">— ${item.artist}</span>`;

      const button = document.createElement('button');
      button.className = 'btn-small flex-shrink-0';
      button.textContent = 'Add';
      // Pass the whole item (title, artist, provider, providerTrackId)
      button.onclick = () => addTrackToPlaylist(item);

      li.appendChild(span);
      li.appendChild(button);
      ul.appendChild(li);
    })
  } else {
    const li = document.createElement('li')
    li.className = 'py-2 text-center text-text-muted'
    li.textContent = 'No results found.'
    ul.appendChild(li)
  }
}

async function addTrackToPlaylist(item) {
  const plIdEl = document.getElementById('pl-id')
  const playlistId = plIdEl ? plIdEl.textContent : null
  
  if (!playlistId) {
    window.showAlert({ title: 'Info', content: 'Playlist ID not found.' })
    return
  }

  const body = {
      title: item.title,
      artist: item.artist,
      provider: item.provider,
      providerTrackId: item.providerTrackId,
      thumbnailUrl: item.thumbnailUrl
  }

  const res = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/tracks`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body)
  })

  if (res.ok) {
    window.showAlert({ title: 'Success', content: 'Track added to playlist!' })
    // Refresh the playlist to show the new track
    loadPlaylist(playlistId)
  } else {
    const errorJson = await res.json()
    window.showAlert({ title: 'Error', content: 'Failed to add track: ' + (errorJson.error || 'Unknown error') })
  }
}

function openCreatePlaylistModal() {
  const content = `
    <div class="space-y-4 text-left">
      <div>
         <label class="block text-sm font-medium text-text-muted mb-1">Name</label>
         <input id="create-pl-name" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-text" placeholder="My Awesome Playlist" />
      </div>
      <div>
         <label class="block text-sm font-medium text-text-muted mb-1">Visibility</label>
         <select id="create-pl-visibility" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-text">
           <option value="true">Public</option>
           <option value="false">Private</option>
         </select>
         <p class="text-xs text-text-muted mt-1">Public: visible to everyone. Private: only visible to you and invited users.</p>
      </div>
      <div>
         <label class="block text-sm font-medium text-text-muted mb-1">Who can edit?</label>
         <select id="create-pl-editmode" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-text">
           <option value="everyone">Everyone</option>
           <option value="invited">Only Invited Users</option>
         </select>
      </div>
    </div>
  `;

  window.showModal({
    title: 'Create Playlist',
    content: content,
    buttons: [
        { text: 'Create', class: 'px-4 py-2 bg-primary hover:bg-primary-hover text-white rounded-md transition-colors', onclick: submitCreatePlaylist },
        { text: 'Cancel', class: 'px-4 py-2 bg-white/5 hover:bg-white/10 text-text-muted rounded-md transition-colors', onclick: window.closeModal }
    ]
  });
}

async function submitCreatePlaylist() {
    const nameInput = document.getElementById('create-pl-name');
    const visInput = document.getElementById('create-pl-visibility');
    const editInput = document.getElementById('create-pl-editmode');
    
    const name = nameInput.value.trim();
    if (!name || name.length < 3) {
        window.showAlert({ title: 'Warning', content: 'Name must be at least 3 characters long.' });
        return;
    }
    
    const isPublic = visInput.value === 'true';
    const editMode = editInput.value;

    const res = await authService.fetchWithAuth(authService.apiUrl + '/playlists', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name, isPublic, editMode })
    });

    const newPlaylist = await res.json();

    if (res.ok) {
        window.closeModal();
        window.showAlert({ title: 'Success', content: 'Playlist created successfully!' });
        if (newPlaylist && newPlaylist.id) {
            window.location.href = '/playlists/' + newPlaylist.id;
        } else {
            refreshPlaylists();
        }
    } else {
        window.showAlert({ title: 'Error', content: 'Failed to create playlist: ' + (newPlaylist.error || 'Unknown error') });
    }
}

async function refreshPlaylists() {
  const container = document.getElementById('playlists-list')
  if (!container) return // Do nothing if container is not present (e.g. detail page)

  const res = await authService.fetchWithAuth(authService.apiUrl + '/playlists')
  if (!res.ok) {
    console.error('Failed to fetch playlists')
    return
  }
  const playlists = await res.json()
  
  container.innerHTML = ''
  
  playlists.forEach(pl => {
    const item = document.createElement('div')
    item.setAttribute('data-id', pl.id)
    item.className = 'relative flex justify-between items-center p-4 bg-white/5 rounded-md hover:bg-white/10 transition-colors'
    
    // Create stretched link
    const link = document.createElement('a')
    link.href = '/playlists/' + pl.id
    link.className = 'absolute inset-0'
    item.appendChild(link)

    const info = document.createElement('div')
    info.className = 'flex-1 min-w-0 pr-4 pointer-events-none'
    
    const name = document.createElement('div')
    name.className = 'font-bold text-text truncate'
    name.textContent = decodeHTMLEntities(pl.name)
    info.appendChild(name)

    const meta = document.createElement('div')
    meta.className = 'text-sm text-text-muted truncate flex gap-2'
    
    // Visibility Badge
    const visBadge = document.createElement('span')
    visBadge.textContent = pl.isPublic ? 'Public' : 'Private'
    visBadge.className = `px-1.5 rounded text-xs ${pl.isPublic ? 'bg-green-500/20 text-green-400' : 'bg-yellow-500/20 text-yellow-400'}`
    meta.appendChild(visBadge)

    // Edit Mode Badge
    const editBadge = document.createElement('span')
    editBadge.textContent = pl.editMode === 'invited' ? 'Restricted Edit' : 'Open Edit'
    editBadge.className = 'px-1.5 rounded text-xs bg-blue-500/20 text-blue-400'
    meta.appendChild(editBadge)
    
    if (pl.description) {
        const desc = document.createElement('span')
        desc.textContent = '• ' + decodeHTMLEntities(pl.description)
        meta.appendChild(desc)
    }

    info.appendChild(meta)
    item.appendChild(info)

    const actions = document.createElement('div')
    actions.className = 'flex items-center gap-4 relative z-10'
    
    // Rename Button
    const btnRename = document.createElement('button')
    btnRename.className = 'btn-small'
    btnRename.textContent = 'Rename'
    btnRename.onclick = () => {
        renamePlaylist(pl.id, pl.name)
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
    btnDelete.onclick = () => {
        deletePlaylist(pl.id)
    }
    actions.appendChild(btnDelete)

    item.appendChild(actions)
    container.appendChild(item)
  })
}

function renamePlaylist(id, currentName) {
  window.showPrompt({
    title: 'Rename Playlist',
    content: 'Enter new name:',
    onConfirm: async (newName) => {
      if (!newName || newName.length < 3) {
        window.showAlert({ title: 'Warning', content: 'Name must be at least 3 characters long.' })
        return
      }
      const res = await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + id, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: newName })
      })
      if (res.ok) {
         // If on list page, refresh
         await refreshPlaylists()
         
         // If on detail page, update name
         const plIdEl = document.getElementById('pl-id')
         if (plIdEl && plIdEl.textContent === id) {
             const plNameEl = document.getElementById('pl-name')
             if (plNameEl) plNameEl.textContent = newName
         }
      } else {
         const err = await res.json().catch(() => ({}))
         window.showAlert({ title: 'Error', content: 'Failed to rename: ' + (err.error || 'Unknown error') })
      }
    }
  })
}

async function deletePlaylist(id) {
    window.showConfirm({
        title: 'Delete Playlist',
        content: 'Are you sure you want to delete this playlist?',
        onConfirm: async () => {
            const res = await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + id, {
                method: 'DELETE'
            })

            if (res.ok) {
                // If on list page, refresh
                await refreshPlaylists()
                
                // If on detail page of the deleted playlist, redirect to list
                const plIdEl = document.getElementById('pl-id')
                if (plIdEl && plIdEl.textContent === id) {
                    window.location.href = '/playlists'
                } else {
                    window.showAlert({ title: 'Success', content: 'Playlist deleted.' })
                }
            } else {
                const err = await res.json().catch(() => ({}))
                window.showAlert({ title: 'Error', content: 'Failed to delete playlist: ' + (err.error || 'Backend does not support deleting playlists yet.') })
            }
        }
    })
}

function highlightPlaylist(playlistId) {
    // Only used on list page, but now logic is separate. 
    // Kept for compatibility if we ever restore split view, but currently unused on detail page.
    const rows = document.querySelectorAll('#playlists-table-body tr')
    rows.forEach(row => {
        if (row.getAttribute('data-id') === playlistId) {
             row.style.backgroundColor = 'rgba(255, 115, 0, 0.2)'
        } else {
             row.style.backgroundColor = ''
        }
    })
}

function decodeHTMLEntities(text) {
    if (!text) return '';
    const textArea = document.createElement('textarea');
    textArea.innerHTML = text;
    return textArea.value;
}

// Load playlists on startup (only if table exists)
refreshPlaylists();

// --- Playlist Settings & Invites ---

var currentPlaylistIdForSettings = null;

window.openPlaylistSettings = async function() {
    // Get ID from the hidden span
    const id = document.getElementById('pl-id').textContent;
    if (!id) return;
    currentPlaylistIdForSettings = id;

    const res = await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + id);
    if (!res.ok) return;
    const data = await res.json();
    const playlist = data.playlist; // response structure is { playlist: ..., tracks: ... }

    const isPrivate = !playlist.isPublic;
    // editMode: "everyone" or "invited"
    const invitesDisplay = (isPrivate || playlist.editMode === 'invited') ? 'block' : 'none';

    // Create Modal Content
    const content = `
      <div class="space-y-4 text-left">
        <div>
           <label class="block text-sm font-medium text-text-muted mb-1">Visibility</label>
           <select id="setting-visibility" onchange="togglePlaylistInvites()" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-text">
             <option value="true" ${!isPrivate ? 'selected' : ''}>Public</option>
             <option value="false" ${isPrivate ? 'selected' : ''}>Private</option>
           </select>
           <p class="text-xs text-text-muted mt-1">Private playlists are only visible to you and invited users.</p>
        </div>

        <div>
           <label class="block text-sm font-medium text-text-muted mb-1">Who can edit?</label>
           <select id="setting-editmode" onchange="togglePlaylistInvites()" class="w-full bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-text">
             <option value="everyone" ${playlist.editMode === 'everyone' ? 'selected' : ''}>Everyone</option>
             <option value="invited" ${playlist.editMode === 'invited' ? 'selected' : ''}>Only Invited Users</option>
           </select>
        </div>
        
        <div id="settings-invites-section" class="border-t border-input-border pt-4" style="display: ${invitesDisplay};">
           <h4 class="text-md font-medium mb-2">Invites</h4>
           <div class="flex gap-2 mb-2">
             <input id="invite-user-id" placeholder="User ID to invite" class="flex-1 bg-input-bg border border-input-border rounded px-2 py-1 focus:outline-none text-sm text-text" />
             <button onclick="sendPlaylistInvite()" class="btn-small">Invite</button>
           </div>
           <ul id="settings-invites-list" class="max-h-40 overflow-y-auto space-y-1 text-sm text-text-muted">
             <li>Loading invites...</li>
           </ul>
        </div>
      </div>
    `;

    window.showModal({
        title: 'Playlist Settings',
        content: content,
        buttons: [
            { text: 'Save Changes', class: 'px-4 py-2 bg-primary hover:bg-primary-hover text-white rounded-md transition-colors', onclick: savePlaylistSettings },
            { text: 'Close', class: 'px-4 py-2 bg-white/5 hover:bg-white/10 text-text-muted rounded-md transition-colors', onclick: window.closeModal }
        ]
    });

    if (isPrivate || playlist.editMode === 'invited') {
        loadPlaylistInvites(id);
    }
}

window.togglePlaylistInvites = function() {
    const vis = document.getElementById('setting-visibility').value; // "true" or "false"
    const edit = document.getElementById('setting-editmode').value; // "everyone" or "invited"
    
    const isPrivate = (vis === "false");
    const isInvitedEdit = (edit === "invited");

    const el = document.getElementById('settings-invites-section');
    if (el) {
        if (isPrivate || isInvitedEdit) {
            el.style.display = 'block';
            loadPlaylistInvites(currentPlaylistIdForSettings);
        } else {
            el.style.display = 'none';
        }
    }
}

async function loadPlaylistInvites(playlistId) {
    const ul = document.getElementById('settings-invites-list');
    if (!ul) return;

    const res = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/invites`);
    if (!res.ok) {
        ul.innerHTML = '<li>Cannot load invites.</li>';
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
           <button onclick="removePlaylistInvite('${inv.userId}')" class="text-error hover:text-red-400">&times;</button>
        `;
        ul.appendChild(li);
    });
}

window.sendPlaylistInvite = async function() {
    const userIdInput = document.getElementById('invite-user-id');
    const userId = userIdInput.value.trim();
    if (!userId) return;

    const res = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${currentPlaylistIdForSettings}/invites`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ userId })
    });

    if (res.ok) {
        userIdInput.value = '';
        loadPlaylistInvites(currentPlaylistIdForSettings);
        window.showToast('User invited.');
    } else {
        const err = await res.json().catch(() => ({}));
        window.showAlert({ title: 'Error', content: err.error || 'Failed to invite user.' });
    }
}

window.removePlaylistInvite = async function(userId) {
     const res = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${currentPlaylistIdForSettings}/invites/${userId}`, {
        method: 'DELETE'
    });
    if (res.ok) {
        loadPlaylistInvites(currentPlaylistIdForSettings);
    } else {
        window.showAlert({ title: 'Error', content: 'Failed to remove invite.' });
    }
}

window.savePlaylistSettings = async function() {
    const isPublic = document.getElementById('setting-visibility').value === 'true';
    const editMode = document.getElementById('setting-editmode').value;
    
    const res = await authService.fetchWithAuth(authService.apiUrl + '/playlists/' + currentPlaylistIdForSettings, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ isPublic, editMode })
    });

    if (res.ok) {
        window.closeModal();
        window.showAlert({ title: 'Success', content: 'Settings updated.' });
        loadPlaylist(currentPlaylistIdForSettings); // Refresh main view
    } else {
        const err = await res.json().catch(() => ({}));
        window.showAlert({ title: 'Error', content: err.error || 'Update failed.' });
    }
}

// --- Drag and Drop ---

var draggedItem = null;

function handleDragStart(e) {
  // console.log('Drag start', this);
  draggedItem = this;
  e.dataTransfer.effectAllowed = 'move';
  e.dataTransfer.setData('text/html', this.innerHTML);
  this.style.opacity = '0.4';
}

function handleDragOver(e) {
  if (e.preventDefault) {
    e.preventDefault();
  }
  e.dataTransfer.dropEffect = 'move';
  return false;
}

function handleDragEnd(e) {
  // console.log('Drag end');
  this.style.opacity = '1';
  draggedItem = null;
}

async function handleDrop(e, playlistId) {
  // console.log('Drop event on', this);
  if (e.stopPropagation) {
    e.stopPropagation();
  }
  
  // Ensure opacity is reset even if logic fails
  if (draggedItem) draggedItem.style.opacity = '1';

  if (draggedItem && draggedItem !== this) {
    const parent = this.parentNode;
    
    // UI Update first (Optimistic)
    const items = Array.from(parent.children);
    const fromIndex = items.indexOf(draggedItem);
    const toIndex = items.indexOf(this);
    
    if (fromIndex < toIndex) {
        parent.insertBefore(draggedItem, this.nextSibling);
    } else {
        parent.insertBefore(draggedItem, this);
    }
    
    const trackId = draggedItem.dataset.trackId;
    
    // Calculate new position
    const newItems = Array.from(parent.children);
    let newPosition = newItems.indexOf(draggedItem);

    if (!trackId || newPosition === -1) {
        console.error('Invalid drop state: missing trackId or item not found');
        loadPlaylist(playlistId);
        return false;
    }
    
    newPosition = parseInt(newPosition, 10);

    console.log(`Moving track ${trackId} to ${newPosition}`);
    
    try {
        const res = await authService.fetchWithAuth(authService.apiUrl + `/playlists/${playlistId}/tracks/${trackId}`, {
            method: 'PATCH',
            headers: { 'content-type': 'application/json' },
            body: JSON.stringify({ newPosition: newPosition })
        });
        
        if (!res.ok) {
            if (res.status === 403) {
                window.showToast('Permission denied: You cannot edit this playlist.');
            } else {
                console.warn('Backend move track failed', res.status);
            }
            loadPlaylist(playlistId);
        }
    } catch (e) {
        console.error('Network or logic error moving track:', e);
        loadPlaylist(playlistId);
    }
  }
  return false;
}

// --- Realtime ---

var playlistWS = null;

function connectPlaylistWS(playlistId) {
    if (playlistWS) {
        playlistWS.close();
        playlistWS = null;
    }
    
    if (typeof WS_URL === 'undefined' || !WS_URL) return;

    playlistWS = new WebSocket(WS_URL);
    
    playlistWS.onopen = () => {
        // console.log('Connected to Playlist WS');
    };
    
    playlistWS.onmessage = (e) => {
        try {
            const msg = JSON.parse(e.data);
            if (msg && msg.payload && msg.payload.playlistId === playlistId) {
                const type = msg.type;
                if (type === 'track.moved' || type === 'track.added' || type === 'track.deleted') {
                    loadPlaylist(playlistId);
                }
            }
        } catch (err) {}
    };
}
