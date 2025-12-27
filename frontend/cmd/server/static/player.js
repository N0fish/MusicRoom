// Ensure global state persists across script re-executions (e.g. HTMX history navigation)
window.mrState = window.mrState || {
    player: null,
    isPlayerReady: false,
    currentPlaylistId: null,
    playerQueue: [],
    playerQueueIndex: 0,
    progressInterval: null,
    isDraggingSlider: false
};

// Initialize YouTube IFrame API
function onYouTubeIframeAPIReady() {
    // If player is already initialized...
    if (window.mrState.player && typeof window.mrState.player.getPlayerState === 'function') {
        try {
            // CRITICAL: Check if the iframe is actually attached to the DOM.
            // If the body was swapped, the old iframe is gone (detached), 
            // even if the JS object persists. We must re-init in that case.
            const iframe = window.mrState.player.getIframe();
            if (iframe && document.body.contains(iframe)) {
                // Player is healthy and in DOM. Skip init to preserve playback.
                console.log("Player active and attached, skipping init.");
                return;
            } else {
                console.warn("Player object exists but iframe is detached/missing. Re-initializing.");
            }
        } catch (e) {
            console.warn("Existing player handle broken, re-initializing:", e);
        }
    }

    // Check if the container exists (it should, unless template is broken)
    if (!document.getElementById('yt-player')) {
        console.warn("yt-player element not found, skipping init.");
        return;
    }

    // Cleanup old instance if needed
    try {
        if (window.mrState.player && typeof window.mrState.player.destroy === 'function') {
            window.mrState.player.destroy();
        }
    } catch (e) {
        console.warn("Error destroying old player:", e);
    }

    window.mrState.player = new YT.Player('yt-player', {
        height: '0',
        width: '0',
        host: 'https://www.youtube.com',
        playerVars: {
            'playsinline': 1,
            'controls': 0,
            'disablekb': 1,
            'enablejsapi': 1,
            'origin': window.location.origin,
            'widget_referrer': window.location.origin
        },
        events: {
            'onReady': onPlayerReady,
            'onStateChange': onPlayerStateChange,
            'onError': onPlayerError
        }
    });
}

function onPlayerReady(event) {
    window.mrState.isPlayerReady = true;
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
    
    window.mrState.playerQueue = tracks;
    window.mrState.playerQueueIndex = startIndex;
    window.mrState.currentPlaylistId = playlistId;
    
    const track = window.mrState.playerQueue[window.mrState.playerQueueIndex];
    loadAndPlay(track);
}

function playNext() {
    if (window.mrState.playerQueue.length === 0) return;
    
    window.mrState.playerQueueIndex++;
    if (window.mrState.playerQueueIndex >= window.mrState.playerQueue.length) {
        return; 
    }
    
    loadAndPlay(window.mrState.playerQueue[window.mrState.playerQueueIndex]);
}

function playPrevious() {
    if (window.mrState.playerQueue.length === 0) return;

    if (window.mrState.player && window.mrState.isPlayerReady) {
        // Safety check
        if (typeof window.mrState.player.getCurrentTime === 'function') {
            const current = window.mrState.player.getCurrentTime();
            if (current > 3) {
                window.mrState.player.seekTo(0, true);
                return;
            }
        }
    }
    
    window.mrState.playerQueueIndex--;
    if (window.mrState.playerQueueIndex < 0) {
        window.mrState.playerQueueIndex = 0;
    }
    
    loadAndPlay(window.mrState.playerQueue[window.mrState.playerQueueIndex]);
}

function loadAndPlay(track) {
    if (!track || !track.providerTrackId) return;

    if (!window.mrState.isPlayerReady) {
        console.error("Player not ready");
        // Try to recover
        if (window.mrState.player && typeof window.mrState.player.loadVideoById === 'function') {
            window.mrState.isPlayerReady = true;
        } else {
            return;
        }
    }

    let displayTitle = decodeHTMLEntities(track.title);
    let artist = decodeHTMLEntities(track.artist);

    if (artist && displayTitle.toLowerCase().startsWith(artist.toLowerCase())) {
        displayTitle = displayTitle.substring(artist.length).replace(/^[\s\-\—\:]+/, '');
    }

    const titleEl = document.getElementById('player-title');
    if (titleEl) titleEl.textContent = displayTitle;
    
    const artistEl = document.getElementById('player-artist');
    if (artistEl) artistEl.textContent = artist;
    
    const infoLink = document.getElementById('player-info-link');
    if (infoLink) {
        if (window.mrState.currentPlaylistId) {
            infoLink.href = '/playlists/' + window.mrState.currentPlaylistId;
            infoLink.style.pointerEvents = 'auto';
        } else {
            infoLink.href = '#';
            infoLink.style.pointerEvents = 'none';
        }
    }
    
    const bar = document.getElementById('player-bar');
    if (bar) bar.classList.remove('translate-y-full');

    try {
        window.mrState.player.loadVideoById(track.providerTrackId);
    } catch (e) {
        console.error("Failed to load video:", e);
    }
}

// ==========================================
// Controls
// ==========================================

function togglePlayPause() {
    if (!window.mrState.player || !window.mrState.isPlayerReady) return;
    
    try {
        const state = window.mrState.player.getPlayerState();
        if (state === YT.PlayerState.PLAYING) {
            window.mrState.player.pauseVideo();
        } else {
            window.mrState.player.playVideo();
        }
    } catch (e) {
        console.error("Error toggling play/pause:", e);
    }
}

function updatePlayerUI(state) {
    const btn = document.getElementById('btn-play-pause');
    if (!btn) return;

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
    window.mrState.progressInterval = setInterval(updateProgress, 1000);
    updateProgress();
}

function stopProgressLoop() {
    if (window.mrState.progressInterval) {
        clearInterval(window.mrState.progressInterval);
        window.mrState.progressInterval = null;
    }
}

function updateProgress() {
    if (!window.mrState.player || !window.mrState.isPlayerReady || window.mrState.isDraggingSlider) return;
    
    // Check if player is accessible
    if (typeof window.mrState.player.getCurrentTime !== 'function') return;

    try {
        const current = window.mrState.player.getCurrentTime();
        const duration = window.mrState.player.getDuration();
        
        if (!duration) return;

        const percent = (current / duration) * 100;
        
        const timeEl = document.getElementById('player-time');
        if (timeEl) timeEl.textContent = formatTime(current) + " / " + formatTime(duration);
        
        const fill = document.getElementById('progress-fill');
        const slider = document.getElementById('progress-slider');
        
        if (fill) fill.style.width = percent + "%";
        if (slider) slider.value = percent;
    } catch (e) {
        // Ignored
    }
}

function handleSeek(percent) {
    window.mrState.isDraggingSlider = true;
    const fill = document.getElementById('progress-fill');
    if (fill) fill.style.width = percent + "%";
    
    if (window.mrState.player && window.mrState.isPlayerReady && typeof window.mrState.player.getDuration === 'function') {
        try {
            const duration = window.mrState.player.getDuration();
            const seekTime = (percent / 100) * duration;
            const timeEl = document.getElementById('player-time');
            if (timeEl) timeEl.textContent = formatTime(seekTime) + " / " + formatTime(duration);
        } catch (e) {}
    }
}

function finishSeek(percent) {
    window.mrState.isDraggingSlider = false;
    if (!window.mrState.player || !window.mrState.isPlayerReady) return;
    
    try {
        const duration = window.mrState.player.getDuration();
        const seekTime = (percent / 100) * duration;
        window.mrState.player.seekTo(seekTime, true);
    } catch (e) {}
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

function getPlayIcon() {
    return `<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`;
}

function getPauseIcon() {
    return `<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`;
}

window.togglePlayPause = togglePlayPause;
window.playPrevious = playPrevious;
window.playNext = playNext;
window.handleSeek = handleSeek;
window.finishSeek = finishSeek;
window.setQueue = setQueue;

if (!window.YT) {
    const tag = document.createElement('script');
    tag.src = "https://www.youtube.com/iframe_api";
    const firstScriptTag = document.getElementsByTagName('script')[0];
    firstScriptTag.parentNode.insertBefore(tag, firstScriptTag);
} else if (window.YT && window.YT.Player) {
    onYouTubeIframeAPIReady();
}