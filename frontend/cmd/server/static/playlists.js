const API = "{{ .API }}"

async function loadPlaylist(playlistId) {
  const res = await authService.fetchWithAuth(API + '/playlists/' + playlistId)
  if (!res.ok) {
    alert('Failed to load playlist.')
    return
  }
  const playlist = await res.json()

  document.getElementById('current-playlist').style.display = 'block'
  document.getElementById('pl-name').textContent = playlist.name
  document.getElementById('pl-id').textContent = playlist.id

  const tracksUl = document.getElementById('pl-tracks')
  tracksUl.innerHTML = ''
  if (playlist.tracks && playlist.tracks.length > 0) {
    playlist.tracks.forEach(track => {
      const li = document.createElement('li')
      li.textContent = `${track.title} — ${track.artist}`
      tracksUl.appendChild(li)
    })
  } else {
    const li = document.createElement('li')
    li.textContent = 'This playlist is empty.'
    tracksUl.appendChild(li)
  }
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
    alert('Please create or load a playlist first.')
    return
  }

  const res = await authService.fetchWithAuth(API + `/playlists/${playlistId}/tracks`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ title, artist })
  })

  if (res.ok) {
    alert('Track added to playlist!')
    // Refresh the playlist to show the new track
    loadPlaylist(playlistId)
  } else {
    const errorJson = await res.json()
    alert('Failed to add track: ' + (errorJson.error || 'Unknown error'))
  }
}

function createPlaylist() {
  window.showPrompt({
    title: 'Create a new playlist',
    content: 'Enter a name for your new playlist:',
    onConfirm: async (name) => {
      if (!name || name.length < 3) {
        alert('Playlist name must be at least 3 characters long.')
        return
      }

      const res = await authService.fetchWithAuth(API + '/playlists', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: name, isPublic: true }) // Make it public by default for now
      })

      if (res.ok) {
        alert('Playlist created successfully!')
        location.reload()
      } else {
        const errorJson = await res.json()
        alert('Failed to create playlist: ' + (errorJson.error || 'Unknown error'))
      }
    }
  })
}