// Global player state
let player;
let isPlayerReady = false;
let currentPlaylistId = null;

// Queue state
let playerQueue = [];
let playerQueueIndex = 0;

// Progress state
let progressInterval = null;
let isDraggingSlider = false;

// Initialize YouTube IFrame API
function onYouTubeIframeAPIReady() {
    player = new YT.Player('yt-player', {
        height: '0',
        width: '0',
        playerVars: {
            'playsinline': 1,
            'controls': 0,
            'disablekb': 1
        },
        events: {
            'onReady': onPlayerReady,
            'onStateChange': onPlayerStateChange,
            'onError': onPlayerError
        }
    });
}

function onPlayerReady(event) {
    isPlayerReady = true;
    updatePlayerUI(YT.PlayerState.PAUSED);
}

function onPlayerStateChange(event) {
    updatePlayerUI(event.data);
    
    if (event.data === YT.PlayerState.PLAYING) {
        startProgressLoop();
    } else {
        stopProgressLoop();
    }
    
    if (event.data === YT.PlayerState.ENDED) {
        playNext();
    }
}

function onPlayerError(event) {
    console.error("Player error:", event.data);
    // 150/101 = restricted/embedded forbidden. Try next.
    if (event.data === 150 || event.data === 101) {
        console.warn("Track not playable, skipping...");
        setTimeout(playNext, 1000);
    }
}

// ==========================================
// Queue Management
// ==========================================

function setQueue(tracks, startIndex = 0, playlistId = null) {
    if (!tracks || tracks.length === 0) return;
    
    playerQueue = tracks;
    playerQueueIndex = startIndex;
    currentPlaylistId = playlistId;
    
    const track = playerQueue[playerQueueIndex];
    loadAndPlay(track);
}

function playNext() {
    if (playerQueue.length === 0) return;
    
    playerQueueIndex++;
    if (playerQueueIndex >= playerQueue.length) {
        // End of queue. Stop.
        return; 
    }
    
    loadAndPlay(playerQueue[playerQueueIndex]);
}

function playPrevious() {
    if (playerQueue.length === 0) return;

    if (player && isPlayerReady) {
        const current = player.getCurrentTime();
        // If played more than 3 seconds, restart track
        if (current > 3) {
            player.seekTo(0, true);
            return;
        }
    }
    
    playerQueueIndex--;
    if (playerQueueIndex < 0) {
        playerQueueIndex = 0;
    }
    
    loadAndPlay(playerQueue[playerQueueIndex]);
}

function loadAndPlay(track) {
    if (!track || !track.providerTrackId) return;

    if (!isPlayerReady) {
        console.error("Player not ready");
        return;
    }

    // Clean title for display
    let displayTitle = decodeHTMLEntities(track.title);
    let artist = decodeHTMLEntities(track.artist);

    if (artist && displayTitle.toLowerCase().startsWith(artist.toLowerCase())) {
        displayTitle = displayTitle.substring(artist.length).replace(/^[\s\-\—\:]+/, '');
    }

    // Update Text UI
    document.getElementById('player-title').textContent = displayTitle;
    document.getElementById('player-artist').textContent = artist;
    
    // Update Playlist Link
    const infoLink = document.getElementById('player-info-link');
    if (infoLink) {
        if (currentPlaylistId) {
            infoLink.href = '/playlists/' + currentPlaylistId;
            infoLink.style.pointerEvents = 'auto';
        } else {
            infoLink.href = '#';
            infoLink.style.pointerEvents = 'none';
        }
    }
    
    // Show Bar
    document.getElementById('player-bar').classList.remove('translate-y-full');

    // Load Video
    player.loadVideoById(track.providerTrackId);
}

// ==========================================
// Controls
// ==========================================

function togglePlayPause() {
    if (!player || !isPlayerReady) return;
    
    const state = player.getPlayerState();
    if (state === YT.PlayerState.PLAYING) {
        player.pauseVideo();
    } else {
        player.playVideo();
    }
}

// Deprecated: kept for compatibility if needed, but setQueue is preferred
function playTrack(providerTrackId, title, artist) {
    // Wrap single track in a queue
    setQueue([{
        providerTrackId: providerTrackId,
        title: title,
        artist: artist
    }], 0);
}

function updatePlayerUI(state) {
    const btn = document.getElementById('btn-play-pause');
    if (state === YT.PlayerState.PLAYING) {
        btn.innerHTML = getPauseIcon();
    } else {
        btn.innerHTML = getPlayIcon();
    }
}

// ==========================================
// Progress & Time
// ==========================================

function startProgressLoop() {
    stopProgressLoop();
    progressInterval = setInterval(updateProgress, 1000);
    updateProgress(); // Immediate update
}

function stopProgressLoop() {
    if (progressInterval) {
        clearInterval(progressInterval);
        progressInterval = null;
    }
}

function updateProgress() {
    if (!player || !isPlayerReady || isDraggingSlider) return;

    const current = player.getCurrentTime();
    const duration = player.getDuration();
    
    if (!duration) return;

    const percent = (current / duration) * 100;
    
    // Update Text
    document.getElementById('player-time').textContent = formatTime(current) + " / " + formatTime(duration);
    
    // Update Slider UI
    const fill = document.getElementById('progress-fill');
    const slider = document.getElementById('progress-slider');
    
    if (fill) fill.style.width = percent + "%";
    if (slider) slider.value = percent;
}

function handleSeek(percent) {
    isDraggingSlider = true;
    const fill = document.getElementById('progress-fill');
    if (fill) fill.style.width = percent + "%";
    
    // Optional: Update time text while dragging
    if (player && isPlayerReady) {
        const duration = player.getDuration();
        const seekTime = (percent / 100) * duration;
        document.getElementById('player-time').textContent = formatTime(seekTime) + " / " + formatTime(duration);
    }
}

function finishSeek(percent) {
    isDraggingSlider = false;
    if (!player || !isPlayerReady) return;
    
    const duration = player.getDuration();
    const seekTime = (percent / 100) * duration;
    
    player.seekTo(seekTime, true);
}

function formatTime(seconds) {
    if (!seconds) return "0:00";
    seconds = Math.floor(seconds);
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return m + ":" + (s < 10 ? "0" : "") + s;
}

function decodeHTMLEntities(text) {
    if (!text) return '';
    const textArea = document.createElement('textarea');
    textArea.innerHTML = text;
    return textArea.value;
}

// ==========================================
// Icons & Setup
// ==========================================

function getPlayIcon() {
    return `<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`;
}

function getPauseIcon() {
    return `<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`;
}

// Load YouTube API script
if (!window.YT) {
    const tag = document.createElement('script');
    tag.src = "https://www.youtube.com/iframe_api";
    const firstScriptTag = document.getElementsByTagName('script')[0];
    firstScriptTag.parentNode.insertBefore(tag, firstScriptTag);
}
