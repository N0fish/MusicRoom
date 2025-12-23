const API = "{{ .API }}"

async function loadPlaylist(playlistId) {
  const res = await authService.fetchWithAuth(API + '/playlists/' + playlistId)
  if (!res.ok) {
    window.showAlert({ title: 'Error', content: 'Failed to load playlist.' })
    return
  }
  const data = await res.json() // The response is PlaylistWithTracks
  const playlist = data.playlist;
  const tracks = data.tracks;

  const currentPlaylistDiv = document.getElementById('current-playlist')
  if (currentPlaylistDiv) currentPlaylistDiv.style.display = 'block'
  
  const plName = document.getElementById('pl-name')
  if (plName) plName.textContent = playlist.name
  
  const plId = document.getElementById('pl-id')
  if (plId) plId.textContent = playlist.id

  const tracksUl = document.getElementById('pl-tracks')
  if (tracksUl) {
      tracksUl.innerHTML = ''
      if (tracks && tracks.length > 0) {
        tracks.forEach(track => {
          const li = document.createElement('li')
          li.className = 'flex justify-between items-center py-3 px-4 bg-white/5 rounded-md hover:bg-white/10 transition-colors mb-2'
          
          const infoDiv = document.createElement('div')
          infoDiv.className = 'flex items-center gap-4 flex-1 min-w-0'

          // Play Button
          if (track.providerTrackId) {
              const btnPlay = document.createElement('button')
              btnPlay.className = 'text-primary hover:text-primary-hover transition-colors flex-shrink-0'
              btnPlay.innerHTML = `
                <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              `
              btnPlay.onclick = () => playTrack(track.providerTrackId, track.title, track.artist)
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
          let displayTitle = track.title;
          if (track.artist && displayTitle.toLowerCase().startsWith(track.artist.toLowerCase())) {
              displayTitle = displayTitle.substring(track.artist.length).replace(/^[\s\-\—\:]+/, '');
          }

          const titleSpan = document.createElement('div')
          titleSpan.className = 'font-bold text-lg truncate'
          titleSpan.textContent = displayTitle
          
          const artistSpan = document.createElement('div')
          artistSpan.className = 'text-sm text-text-muted truncate'
          artistSpan.textContent = track.artist
          
          textDiv.appendChild(titleSpan)
          textDiv.appendChild(artistSpan)
          infoDiv.appendChild(textDiv)
          
          li.appendChild(infoDiv)

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

          tracksUl.appendChild(li)
        })
      } else {
        const li = document.createElement('li')
        li.className = 'py-4 text-center text-text-muted'
        li.textContent = 'This playlist is empty.'
        tracksUl.appendChild(li)
      }
  }
}

async function deleteTrack(playlistId, trackId) {
    window.showConfirm({
        title: 'Remove Track',
        content: 'Are you sure you want to remove this track?',
        onConfirm: async () => {
            const res = await authService.fetchWithAuth(API + `/playlists/${playlistId}/tracks/${trackId}`, {
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

  const res = await authService.fetchWithAuth(API + '/music/search?query=' + encodeURIComponent(query))
  const json = await res.json()

  const ul = document.getElementById('music-search-results')
  ul.innerHTML = ''

  if (json.items && json.items.length > 0) {
    json.items.forEach(item => {
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

  const res = await authService.fetchWithAuth(API + `/playlists/${playlistId}/tracks`, {
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

function createPlaylist() {
  window.showPrompt({
    title: 'Create a new playlist',
    content: 'Enter a name for your new playlist:',
    onConfirm: async (name) => {
      if (!name || name.length < 3) {
        window.showAlert({ title: 'Warning', content: 'Playlist name must be at least 3 characters long.' })
        return
      }

      const res = await authService.fetchWithAuth(API + '/playlists', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: name, isPublic: true }) // Make it public by default for now
      })

      const newPlaylist = await res.json()
      
      if (res.ok) {
        window.showAlert({ title: 'Success', content: 'Playlist created successfully!' })
        
        // Redirect to the new playlist detail page
        if (newPlaylist && newPlaylist.id) {
            window.location.href = '/playlists/' + newPlaylist.id;
        } else {
            // Fallback if ID missing (shouldn't happen)
            refreshPlaylists();
        }
      } else {
        const errorJson = newPlaylist 
        window.showAlert({ title: 'Error', content: 'Failed to create playlist: ' + (errorJson.error || 'Unknown error') })
      }
    }
  })
}

async function refreshPlaylists() {
  const container = document.getElementById('playlists-list')
  if (!container) return // Do nothing if container is not present (e.g. detail page)

  const res = await authService.fetchWithAuth(API + '/playlists')
  if (!res.ok) {
    console.error('Failed to fetch playlists')
    return
  }
  const playlists = await res.json()
  
  container.innerHTML = ''
  
  playlists.forEach(pl => {
    const item = document.createElement('div')
    item.setAttribute('data-id', pl.id)
    item.className = 'flex justify-between items-center p-4 bg-white/5 rounded-md cursor-pointer hover:bg-white/10 transition-colors'
    
    // Navigate to detail page on click
    item.onclick = () => {
        window.location.href = '/playlists/' + pl.id
    }

    const info = document.createElement('div')
    info.className = 'flex-1 min-w-0 pr-4'
    
    const name = document.createElement('div')
    name.className = 'font-bold text-text truncate'
    name.textContent = pl.name
    info.appendChild(name)

    const desc = document.createElement('div')
    desc.className = 'text-sm text-text-muted truncate'
    desc.textContent = pl.description || ''
    info.appendChild(desc)
    
    item.appendChild(info)

    const actions = document.createElement('div')
    actions.className = 'flex items-center gap-4'
    
    // Rename Button
    const btnRename = document.createElement('button')
    btnRename.className = 'btn-small'
    btnRename.textContent = 'Rename'
    btnRename.onclick = (e) => {
        e.stopPropagation()
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
    btnDelete.onclick = (e) => {
        e.stopPropagation()
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
      const res = await authService.fetchWithAuth(API + '/playlists/' + id, {
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
            const res = await authService.fetchWithAuth(API + '/playlists/' + id, {
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

// Load playlists on startup (only if table exists)
refreshPlaylists();
