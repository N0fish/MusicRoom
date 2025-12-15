
const API = "{{ .API }}"
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
      li.innerHTML = `
        <span>${item.title} — ${item.artist}</span>
        <button class="btn btn-sm" onclick="addTrackToPlaylist('${item.title}', '${item.artist}')">Add</button>
      `
      ul.appendChild(li)
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
  } else {
    const errorJson = await res.json()
    alert('Failed to add track: ' + (errorJson.error || 'Unknown error'))
  }
}
