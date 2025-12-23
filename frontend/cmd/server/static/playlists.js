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

  document.getElementById('current-playlist').style.display = 'block'
  document.getElementById('pl-name').textContent = playlist.name
  document.getElementById('pl-id').textContent = playlist.id

  const tracksUl = document.getElementById('pl-tracks')
  tracksUl.innerHTML = ''
  if (tracks && tracks.length > 0) {
    tracks.forEach(track => {
      const li = document.createElement('li')
      li.textContent = `${track.title} — ${track.artist}`
      tracksUl.appendChild(li)
    })
  } else {
    const li = document.createElement('li')
    li.textContent = 'This playlist is empty.'
    tracksUl.appendChild(li)
  }
  
  highlightPlaylist(playlistId)
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
      li.className = 'flex justify-between items-center py-1'

      const span = document.createElement('span');
      span.textContent = `${item.title} — ${item.artist}`;

      const button = document.createElement('button');
      button.className = 'btn btn-sm';
      button.textContent = 'Add';
      button.onclick = () => addTrackToPlaylist(item.title, item.artist);

      li.appendChild(span);
      li.appendChild(button);
      ul.appendChild(li);
    })
  } else {
    const li = document.createElement('li')
    li.textContent = 'No results found.'
    ul.appendChild(li)
  }
}

async function addTrackToPlaylist(title, artist) {
  const playlistId = document.getElementById('pl-id').textContent
  if (!playlistId) {
    window.showAlert({ title: 'Info', content: 'Please create or load a playlist first.' })
    return
  }

  const res = await authService.fetchWithAuth(API + `/playlists/${playlistId}/tracks`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ title, artist })
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
      console.log('New Playlist created response:', newPlaylist); // Debug log

      if (res.ok) {
        window.showAlert({ title: 'Success', content: 'Playlist created successfully!' })
        
        // Refresh the list so the new playlist appears
        await refreshPlaylists()

        // Auto-load the newly created playlist
        if (newPlaylist && newPlaylist.id) {
            loadPlaylist(newPlaylist.id);
        }
      } else {
        const errorJson = newPlaylist // Error is already parsed here
        window.showAlert({ title: 'Error', content: 'Failed to create playlist: ' + (errorJson.error || 'Unknown error') })
      }
    }
  })
}

async function refreshPlaylists() {
  const res = await authService.fetchWithAuth(API + '/playlists')
  if (!res.ok) {
    console.error('Failed to fetch playlists')
    return
  }
  const playlists = await res.json()
  const tbody = document.getElementById('playlists-table-body')
  if (!tbody) return
  tbody.innerHTML = ''
  
  const currentId = document.getElementById('pl-id').textContent

  playlists.forEach(pl => {
    const tr = document.createElement('tr')
    tr.setAttribute('data-id', pl.id)
    tr.className = 'cursor-pointer hover:bg-white/5 transition-colors'
    tr.onclick = () => loadPlaylist(pl.id)

    if (currentId === pl.id) {
        tr.style.backgroundColor = 'rgba(255, 115, 0, 0.2)'
    }

    const tdName = document.createElement('td')
    tdName.className = 'py-2 px-4 border-b'
    tdName.textContent = pl.name
    tr.appendChild(tdName)

    const tdDesc = document.createElement('td')
    tdDesc.className = 'py-2 px-4 border-b'
    tdDesc.textContent = pl.description || ''
    tr.appendChild(tdDesc)

    const tdAction = document.createElement('td')
    tdAction.className = 'py-2 px-4 border-b text-right'
    
    // Rename Button
    const btnRename = document.createElement('button')
    btnRename.className = 'btn-small mr-2'
    btnRename.textContent = 'Rename'
    btnRename.onclick = (e) => {
        e.stopPropagation()
        renamePlaylist(pl.id, pl.name)
    }
    tdAction.appendChild(btnRename)

    // Delete Button (Trash Icon)
    const btnDelete = document.createElement('button')
    btnDelete.className = 'text-primary hover:text-primary-hover transition-colors'
    btnDelete.title = 'Delete'
    btnDelete.innerHTML = `
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
    `
    btnDelete.onclick = (e) => {
        e.stopPropagation()
        deletePlaylist(pl.id)
    }
    tdAction.appendChild(btnDelete)

    tr.appendChild(tdAction)
    tbody.appendChild(tr)
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
         await refreshPlaylists()
         const currentId = document.getElementById('pl-id').textContent
         if (currentId === id) {
             document.getElementById('pl-name').textContent = newName
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
                await refreshPlaylists()
                // If the deleted playlist was the active one, clear the view
                const currentId = document.getElementById('pl-id').textContent
                if (currentId === id) {
                    document.getElementById('current-playlist').style.display = 'none'
                    document.getElementById('pl-id').textContent = ''
                    document.getElementById('pl-name').textContent = ''
                    document.getElementById('pl-tracks').innerHTML = ''
                }
                window.showAlert({ title: 'Success', content: 'Playlist deleted.' })
            } else {
                const err = await res.json().catch(() => ({}))
                // Note: DELETE might not be implemented in the backend yet.
                window.showAlert({ title: 'Error', content: 'Failed to delete playlist: ' + (err.error || 'Backend does not support deleting playlists yet.') })
            }
        }
    })
}

function highlightPlaylist(playlistId) {
    const rows = document.querySelectorAll('#playlists-table-body tr')
    rows.forEach(row => {
        if (row.getAttribute('data-id') === playlistId) {
             row.style.backgroundColor = 'rgba(255, 115, 0, 0.2)'
        } else {
             row.style.backgroundColor = ''
        }
    })
}

// Load playlists on startup
refreshPlaylists();
