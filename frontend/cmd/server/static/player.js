// Global player state
let player;
let isPlayerReady = false;
let currentTrackId = null;

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
            'onStateChange': onPlayerStateChange
        }
    });
}

function onPlayerReady(event) {
    isPlayerReady = true;
    console.log("Player ready");
}

function onPlayerStateChange(event) {
    updatePlayerUI(event.data);
}

// Controls
function playTrack(providerTrackId, title, artist) {
    if (!isPlayerReady || !providerTrackId) {
        console.error("Player not ready or invalid ID");
        return;
    }

    // Load and play video
    player.loadVideoById(providerTrackId);
    
    // Update UI
    document.getElementById('player-bar').classList.remove('translate-y-full');
    document.getElementById('player-title').textContent = title;
    document.getElementById('player-artist').textContent = artist;
    document.getElementById('btn-play-pause').innerHTML = getPauseIcon();
}

function togglePlayPause() {
    if (!player || !isPlayerReady) return;
    
    const state = player.getPlayerState();
    if (state === YT.PlayerState.PLAYING) {
        player.pauseVideo();
    } else {
        player.playVideo();
    }
}

function updatePlayerUI(state) {
    const btn = document.getElementById('btn-play-pause');
    if (state === YT.PlayerState.PLAYING) {
        btn.innerHTML = getPauseIcon();
    } else {
        btn.innerHTML = getPlayIcon();
    }
}

// Icons
function getPlayIcon() {
    return `<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`;
}

function getPauseIcon() {
    return `<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`;
}

// Load YouTube API script
const tag = document.createElement('script');
tag.src = "https://www.youtube.com/iframe_api";
const firstScriptTag = document.getElementsByTagName('script')[0];
firstScriptTag.parentNode.insertBefore(tag, firstScriptTag);
