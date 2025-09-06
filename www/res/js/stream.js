// Global variables - prevent redeclaration errors on HTMX navigation
window.streamVideo = window.streamVideo || null;
window.streamHls = window.streamHls || null;
window.currentStatus = window.currentStatus || 'offline';
window.updateInterval = window.updateInterval || null;

// Initialize when DOM loads OR when HTMX content is swapped in
document.addEventListener('DOMContentLoaded', function() {
    window.initializeStream();
    window.startStatusUpdates();
    window.loadHomepagePastStreams();
});

// Also initialize when HTMX swaps in new content (for SPA navigation)
document.addEventListener('htmx:afterSettle', function(evt) {
    // Only initialize if we're on the live stream page
    if (document.getElementById('videoPlayer')) {
        window.initializeStream();
        window.startStatusUpdates();
        window.loadHomepagePastStreams();
    }
});

window.initializeStream = window.initializeStream || function() {
    window.streamVideo = document.getElementById('videoPlayer');
    if (!window.streamVideo) return;
    
    // Try to load live stream initially
    window.loadLiveStream();
}

window.startStatusUpdates = window.startStatusUpdates || function() {
    if (window.updateInterval) {
        clearInterval(window.updateInterval);
    }
    window.updateInterval = setInterval(window.updateStreamData, 10000); // Every 10 seconds
}

window.updateStreamData = window.updateStreamData || async function() {
    try {
        const response = await fetch('/api/stream-data');
        const data = await response.json();
        
        // Handle new response format with metadata wrapper
        const metadata = data.metadata || data;
        const viewerCount = data.active_viewers || 0;
        
        const newStatus = metadata.status?.toLowerCase() || 'offline';
        
        // Update status display
        window.updateStatusDisplay(newStatus, viewerCount);
        
        // Update metadata
        window.updateStreamInfo(metadata);
        
        // Handle status changes
        if (newStatus !== window.currentStatus) {
            console.log(`Status changed: ${window.currentStatus} -> ${newStatus}`);
            window.currentStatus = newStatus;
            
            if (newStatus === 'live' && metadata.stream_url) {
                window.loadStream(metadata.stream_url);
            }
        }
        
    } catch (error) {
        console.error('Failed to update stream data:', error);
    }
}

window.updateStatusDisplay = window.updateStatusDisplay || function(status, viewerCount = 0) {
    const statusEl = document.getElementById('streamStatus');
    if (!statusEl) return;
    
    const viewerText = status === 'live' ? `<span class="ml-2 text-sm opacity-75">[${viewerCount} VIEWERS]</span>` : '<span class="ml-2 text-sm opacity-75">[NODE_ACTIVE]</span>';
    
    statusEl.innerHTML = `<span class="mr-2">◉</span>${status.toUpperCase()}${viewerText}`;
    statusEl.className = 'px-8 py-4 text-xl font-bold rounded-md font-mono uppercase tracking-widest cyber-title';
    
    if (status === 'live') {
        statusEl.classList.add('status-live');
    } else {
        statusEl.classList.add('status-offline');
    }
}

window.updateStreamInfo = window.updateStreamInfo || function(data) {
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

window.loadLiveStream = window.loadLiveStream || function() {
    window.loadStream('/live/output.m3u8');
}

window.loadStream = window.loadStream || function(streamUrl) {
    if (!window.streamVideo) return;
    
    console.log('Loading stream:', streamUrl);
    
    if (window.streamHls) {
        window.streamHls.destroy();
        window.streamHls = null;
    }
    
    if (Hls.isSupported()) {
        window.streamHls = new Hls({
            enableWorker: true,
            lowLatencyMode: true,
        });
        
        window.streamHls.loadSource(streamUrl);
        window.streamHls.attachMedia(window.streamVideo);
        
        window.streamHls.on(Hls.Events.MANIFEST_PARSED, function() {
            console.log('Stream loaded successfully');
        });
        
        window.streamHls.on(Hls.Events.ERROR, function(event, data) {
            console.error('HLS Error:', data);
        });
    } else if (window.streamVideo.canPlayType('application/vnd.apple.mpegurl')) {
        window.streamVideo.src = streamUrl;
    }
}

window.refreshStream = window.refreshStream || function() {
    window.updateStreamData();
    if (window.currentStatus === 'live') {
        window.loadLiveStream();
    }
}

window.toggleFullscreen = window.toggleFullscreen || function() {
    if (!window.streamVideo) return;
    
    if (document.fullscreenElement) {
        document.exitFullscreen();
    } else {
        window.streamVideo.requestFullscreen();
    }
}

// Past streams functionality
window.loadPastStreams = window.loadPastStreams || async function() {
    const loadingEl = document.getElementById('pastStreamsLoading');
    const errorEl = document.getElementById('pastStreamsError');
    const gridEl = document.getElementById('pastStreamsGrid');
    const emptyEl = document.getElementById('pastStreamsEmpty');
    
    if (loadingEl) loadingEl.style.display = 'block';
    if (errorEl) errorEl.classList.add('hidden');
    if (gridEl) gridEl.classList.add('hidden');
    if (emptyEl) emptyEl.classList.add('hidden');
    
    try {
        const response = await fetch('/archive/');
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
                    const metaResponse = await fetch(`/archive/${folderPath}/metadata.json`);
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
        const streamsHtml = validStreams.map(window.createStreamCard).join('');
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

window.createStreamCard = window.createStreamCard || function(stream) {
    const date = new Date(parseInt(stream.starts) * 1000).toLocaleDateString();
    const duration = stream.ends ? 
        Math.round((parseInt(stream.ends) - parseInt(stream.starts)) / 60) + ' min' : 
        'UNKNOWN_DURATION';
    
    return `
        <div class="terminal-box rounded-md p-4 cursor-pointer transition-all transform hover:scale-105 hover:shadow-lg hover:shadow-green-500/20"
             onclick="window.selectPastStream('${stream.folderPath}', '${stream.recording_url}')">
            <!-- Terminal Header -->
            <div class="flex items-center text-xs text-cyan-400 font-mono mb-3">
                <span>STREAM_${stream.folderPath.slice(-6)}.dat</span>
                <span class="ml-auto text-green-400">●</span>
            </div>
            
            <!-- Thumbnail -->
            ${stream.image ? `
                <div class="video-frame rounded-md mb-3 overflow-hidden" style="aspect-ratio: 16/9;">
                    <img src="${stream.image}" 
                         alt="Stream thumbnail" 
                         class="w-full h-full object-cover bg-black"
                         onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
                    <div class="hidden w-full h-full bg-black flex items-center justify-center text-cyan-400">
                        <span class="font-mono text-sm">NO_PREVIEW</span>
                    </div>
                </div>
            ` : `
                <div class="video-frame rounded-md mb-3 bg-black flex items-center justify-center text-cyan-400" style="aspect-ratio: 16/9;">
                    <span class="font-mono text-sm">NO_THUMBNAIL</span>
                </div>
            `}
            
            <!-- Stream Info -->
            <h4 class="font-bold mb-2 text-green-400 cyber-title">${stream.title || '[UNTITLED_STREAM]'}</h4>
            <p class="text-cyan-300 text-sm mb-2 font-mono line-clamp-2">${stream.summary || 'No neural data available'}</p>
            
            <!-- Metadata -->
            <div class="flex justify-between items-center text-xs text-gray-500 font-mono mb-2">
                <span>TIMESTAMP: ${date}</span>
                <span>DURATION: ${duration}</span>
            </div>
            
            <!-- Tags -->
            ${stream.tags && stream.tags.length > 0 ? `
                <div class="flex flex-wrap gap-1 mt-3">
                    ${stream.tags.slice(0, 3).map(tag => 
                        `<span class="neon-border text-green-400 px-2 py-1 text-xs rounded font-mono">#${tag}</span>`
                    ).join('')}
                    ${stream.tags.length > 3 ? `<span class="text-gray-500 px-2 py-1 text-xs font-mono">+${stream.tags.length - 3}</span>` : ''}
                </div>
            ` : ''}
        </div>
    `;
}

window.selectPastStream = window.selectPastStream || function(folderPath, recordingUrl) {
    console.log('Loading past stream:', folderPath);
    window.loadStream(recordingUrl);
    
    // Scroll to video
    window.streamVideo?.scrollIntoView({ behavior: 'smooth', block: 'center' });
}

// Homepage past streams functionality (shows recent 6 streams)
window.loadHomepagePastStreams = window.loadHomepagePastStreams || async function() {
    const loadingEl = document.getElementById('pastStreamsLoading');
    const gridEl = document.getElementById('pastStreamsGrid');
    const emptyEl = document.getElementById('pastStreamsEmpty');
    
    if (!loadingEl) return; // Not on live page
    
    try {
        // Same approach as archive.js - get directory listing
        const response = await fetch('/archive/');
        const directoryHtml = await response.text();
        
        const parser = new DOMParser();
        const doc = parser.parseFromString(directoryHtml, 'text/html');
        const links = Array.from(doc.querySelectorAll('a'))
            .map(a => a.textContent.trim())
            .filter(name => /^\d{1,2}-\d{1,2}-\d{4}-\d{6}\/?$/.test(name));
        
        if (loadingEl) loadingEl.style.display = 'none';
        
        if (links.length === 0) {
            if (emptyEl) emptyEl.classList.remove('hidden');
            return;
        }
        
        // Load metadata for streams (limit to 6 most recent)
        const recentLinks = links.slice(0, 6);
        const streams = await Promise.all(
            recentLinks.map(async (folder) => {
                try {
                    const folderPath = folder.replace(/\/$/, '');
                    const metaResponse = await fetch(`/archive/${folderPath}/metadata.json`);
                    if (!metaResponse.ok) throw new Error('No metadata');
                    
                    const metadata = await metaResponse.json();
                    return {
                        ...metadata,
                        folderPath: folderPath,
                        recording_url: `/archive/${folderPath}/output.m3u8`
                    };
                } catch (error) {
                    console.error(`Failed to load metadata for ${folder}:`, error);
                    return null;
                }
            })
        );
        
        // Filter out failed loads
        const validStreams = streams.filter(s => s !== null);
        
        if (validStreams.length === 0) {
            if (emptyEl) emptyEl.classList.remove('hidden');
            return;
        }
        
        const cardsHtml = validStreams.map(window.createStreamCard).join('');
        if (gridEl) {
            gridEl.innerHTML = cardsHtml;
            gridEl.classList.remove('hidden');
        }
        
    } catch (error) {
        console.error('Error loading homepage past streams:', error);
        if (loadingEl) loadingEl.style.display = 'none';
        if (emptyEl) emptyEl.classList.remove('hidden');
    }
};

// Cleanup on page unload
window.addEventListener('beforeunload', function() {
    if (window.updateInterval) {
        clearInterval(window.updateInterval);
    }
    if (window.streamHls) {
        window.streamHls.destroy();
    }
});