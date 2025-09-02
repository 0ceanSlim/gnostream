// Global variables
let video = document.getElementById('videoPlayer');
let hls = null;
let currentStatus = 'offline';
let updateInterval = null;

// Initialize when DOM loads
document.addEventListener('DOMContentLoaded', function() {
    initializeStream();
    startStatusUpdates();
    loadPastStreams();
});

function initializeStream() {
    video = document.getElementById('videoPlayer');
    if (!video) return;
    
    // Try to load live stream initially
    loadLiveStream();
}

function startStatusUpdates() {
    updateInterval = setInterval(updateStreamData, 10000); // Every 10 seconds
}

async function updateStreamData() {
    try {
        const response = await fetch('/api/stream-data');
        const data = await response.json();
        
        const newStatus = data.status?.toLowerCase() || 'offline';
        
        // Update status display
        updateStatusDisplay(newStatus);
        
        // Update metadata
        updateStreamInfo(data);
        
        // Handle status changes
        if (newStatus !== currentStatus) {
            console.log(`Status changed: ${currentStatus} -> ${newStatus}`);
            currentStatus = newStatus;
            
            if (newStatus === 'live' && data.stream_url) {
                loadStream(data.stream_url);
            }
        }
        
    } catch (error) {
        console.error('Failed to update stream data:', error);
    }
}

function updateStatusDisplay(status) {
    const statusEl = document.getElementById('streamStatus');
    if (!statusEl) return;
    
    statusEl.innerHTML = `<span class="mr-2">◉</span>${status.toUpperCase()}<span class="ml-2 text-sm opacity-75">[NODE_ACTIVE]</span>`;
    statusEl.className = 'px-8 py-4 text-xl font-bold rounded-md font-mono uppercase tracking-widest cyber-title';
    
    if (status === 'live') {
        statusEl.classList.add('status-live');
    } else {
        statusEl.classList.add('status-offline');
    }
}

function updateStreamInfo(data) {
    const titleEl = document.getElementById('streamTitle');
    const summaryEl = document.getElementById('streamSummary');
    const tagsEl = document.getElementById('streamTags');
    
    if (titleEl && data.title) titleEl.textContent = data.title;
    if (summaryEl && data.summary) summaryEl.textContent = data.summary;
    
    if (tagsEl && data.tags) {
        tagsEl.innerHTML = data.tags.map(tag => 
            `<span class="neon-border text-green-400 px-3 py-1 rounded text-sm font-mono uppercase">#${tag}</span>`
        ).join('');
    }
}

function loadLiveStream() {
    loadStream('/live/output.m3u8');
}

function loadStream(streamUrl) {
    if (!video) return;
    
    console.log('Loading stream:', streamUrl);
    
    if (hls) {
        hls.destroy();
        hls = null;
    }
    
    if (Hls.isSupported()) {
        hls = new Hls({
            enableWorker: true,
            lowLatencyMode: true,
        });
        
        hls.loadSource(streamUrl);
        hls.attachMedia(video);
        
        hls.on(Hls.Events.MANIFEST_PARSED, function() {
            console.log('Stream loaded successfully');
        });
        
        hls.on(Hls.Events.ERROR, function(event, data) {
            console.error('HLS Error:', data);
        });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
        video.src = streamUrl;
    }
}

function refreshStream() {
    updateStreamData();
    if (currentStatus === 'live') {
        loadLiveStream();
    }
}

function toggleFullscreen() {
    if (!video) return;
    
    if (document.fullscreenElement) {
        document.exitFullscreen();
    } else {
        video.requestFullscreen();
    }
}

// Past streams functionality
async function loadPastStreams() {
    const loadingEl = document.getElementById('pastStreamsLoading');
    const errorEl = document.getElementById('pastStreamsError');
    const gridEl = document.getElementById('pastStreamsGrid');
    const emptyEl = document.getElementById('pastStreamsEmpty');
    
    if (loadingEl) loadingEl.style.display = 'block';
    if (errorEl) errorEl.classList.add('hidden');
    if (gridEl) gridEl.classList.add('hidden');
    if (emptyEl) emptyEl.classList.add('hidden');
    
    try {
        const response = await fetch('/past-streams/');
        const html = await response.text();
        
        const parser = new DOMParser();
        const doc = parser.parseFromString(html, 'text/html');
        const links = Array.from(doc.querySelectorAll('a'))
            .map(a => a.textContent.trim())
            .filter(name => /^\d{1,2}-\d{1,2}-\d{4}-\d{6}\/?$/.test(name));
        
        if (loadingEl) loadingEl.style.display = 'none';
        
        if (links.length === 0) {
            if (emptyEl) emptyEl.classList.remove('hidden');
            return;
        }
        
        // Load metadata for each stream
        const streams = await Promise.all(
            links.map(async (folder) => {
                try {
                    const folderPath = folder.replace(/\/$/, '');
                    const metaResponse = await fetch(`/past-streams/${folderPath}/metadata.json`);
                    if (!metaResponse.ok) throw new Error('No metadata');
                    
                    const metadata = await metaResponse.json();
                    metadata.folderPath = folderPath;
                    return metadata;
                } catch {
                    return null;
                }
            })
        );
        
        const validStreams = streams.filter(s => s !== null);
        
        if (validStreams.length === 0) {
            if (emptyEl) emptyEl.classList.remove('hidden');
            return;
        }
        
        // Generate HTML for streams
        const streamsHtml = validStreams.map(createStreamCard).join('');
        if (gridEl) {
            gridEl.innerHTML = streamsHtml;
            gridEl.classList.remove('hidden');
        }
        
    } catch (error) {
        console.error('Error loading past streams:', error);
        if (loadingEl) loadingEl.style.display = 'none';
        if (errorEl) errorEl.classList.remove('hidden');
    }
}

function createStreamCard(stream) {
    const date = new Date(parseInt(stream.starts) * 1000).toLocaleDateString();
    
    return `
        <div class="terminal-box rounded-md p-4 cursor-pointer transition-all transform hover:scale-105 hover:shadow-lg hover:shadow-green-500/20"
             onclick="selectPastStream('${stream.folderPath}', '${stream.recording_url}')">
            <div class="flex items-center text-xs text-cyan-400 font-mono mb-2">
                <span>STREAM_${stream.folderPath.slice(-6)}.dat</span>
                <span class="ml-auto text-green-400">●</span>
            </div>
            <h4 class="font-bold mb-2 text-green-400 cyber-title">${stream.title || '[UNTITLED_STREAM]'}</h4>
            <p class="text-cyan-300 text-sm mb-2 font-mono">${stream.summary || 'No neural data available'}</p>
            <p class="text-xs text-gray-500 font-mono">TIMESTAMP: ${date}</p>
            ${stream.tags ? `
                <div class="flex flex-wrap gap-1 mt-3">
                    ${stream.tags.slice(0, 3).map(tag => 
                        `<span class="neon-border text-green-400 px-2 py-1 text-xs rounded font-mono">#${tag}</span>`
                    ).join('')}
                </div>
            ` : ''}
        </div>
    `;
}

function selectPastStream(folderPath, recordingUrl) {
    console.log('Loading past stream:', folderPath);
    loadStream(recordingUrl);
    
    // Scroll to video
    video?.scrollIntoView({ behavior: 'smooth', block: 'center' });
}

// Cleanup on page unload
window.addEventListener('beforeunload', function() {
    if (updateInterval) {
        clearInterval(updateInterval);
    }
    if (hls) {
        hls.destroy();
    }
});