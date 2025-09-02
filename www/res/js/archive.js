let archiveData = [];
let currentHls = null;

document.addEventListener('DOMContentLoaded', function() {
    loadArchive();
});

async function loadArchive() {
    const loadingEl = document.getElementById('archiveLoading');
    const gridEl = document.getElementById('archiveGrid');
    const emptyEl = document.getElementById('archiveEmpty');
    
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
        
        // Load metadata for streams
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
        
        archiveData = streams.filter(s => s !== null)
            .sort((a, b) => parseInt(b.starts || 0) - parseInt(a.starts || 0));
        
        if (archiveData.length === 0) {
            if (emptyEl) emptyEl.classList.remove('hidden');
            return;
        }
        
        renderArchive();
        
    } catch (error) {
        console.error('Error loading archive:', error);
        if (loadingEl) loadingEl.style.display = 'none';
    }
}

function renderArchive() {
    const gridEl = document.getElementById('archiveGrid');
    if (!gridEl || archiveData.length === 0) return;
    
    const html = archiveData.map(createArchiveCard).join('');
    gridEl.innerHTML = html;
    gridEl.classList.remove('hidden');
}

function createArchiveCard(stream) {
    const date = new Date(parseInt(stream.starts) * 1000);
    const duration = stream.ends ? 
        Math.round((parseInt(stream.ends) - parseInt(stream.starts)) / 60) + ' min' : 
        'UNKNOWN_DURATION';
    
    return `
        <div class="terminal-box rounded-lg p-6 cursor-pointer transition-all transform hover:scale-105 hover:shadow-lg hover:shadow-cyan-500/20"
             onclick="openStreamModal('${stream.folderPath}')">
            <!-- Terminal Header -->
            <div class="flex items-center text-xs text-cyan-400 font-mono mb-3">
                <span>NEURAL_${stream.folderPath.slice(-6)}.stream</span>
                <span class="ml-auto text-green-400">◉</span>
            </div>
            
            <!-- Video Preview -->
            <div class="aspect-video neon-border rounded-md mb-4 flex items-center justify-center relative overflow-hidden">
                ${stream.image ? 
                    `<img src="${stream.image}" alt="${stream.title}" class="w-full h-full object-cover rounded-md">` :
                    '<div class="text-6xl text-cyan-400">◉</div>'
                }
                <div class="absolute inset-0 flex items-center justify-center opacity-0 hover:opacity-100 transition-opacity bg-black bg-opacity-70 rounded-md">
                    <div class="text-center">
                        <div class="text-4xl text-green-400 mb-2">▶</div>
                        <div class="text-sm text-cyan-400 font-mono">LOAD_STREAM</div>
                    </div>
                </div>
            </div>
            
            <h3 class="font-bold text-lg mb-2 text-green-400 cyber-title neon-glow-subtle">
                ${stream.title || '[UNTITLED_NEURAL_STREAM]'}
            </h3>
            
            <p class="text-cyan-300 text-sm mb-3 font-mono leading-relaxed">
                ${stream.summary || 'No consciousness data available in neural manifest'}
            </p>
            
            <!-- Metadata -->
            <div class="flex justify-between items-center text-xs font-mono mb-3">
                <span class="text-gray-400">TIMESTAMP: ${date.toLocaleDateString()}</span>
                <span class="text-green-400">DURATION: ${duration}</span>
            </div>
            
            <!-- Neural Tags -->
            ${stream.tags && stream.tags.length > 0 ? `
                <div class="flex flex-wrap gap-1">
                    ${stream.tags.slice(0, 4).map(tag => 
                        `<span class="neon-border text-green-400 px-2 py-1 text-xs rounded font-mono">#${tag}</span>`
                    ).join('')}
                    ${stream.tags.length > 4 ? `<span class="text-xs text-cyan-400 font-mono">+${stream.tags.length - 4} MORE</span>` : ''}
                </div>
            ` : ''}
        </div>
    `;
}

function openStreamModal(folderPath) {
    const stream = archiveData.find(s => s.folderPath === folderPath);
    if (!stream) return;
    
    const modal = document.getElementById('streamModal');
    const title = document.getElementById('modalTitle');
    const date = document.getElementById('modalDate');
    const video = document.getElementById('modalVideo');
    const summary = document.getElementById('modalSummary');
    const tags = document.getElementById('modalTags');
    
    if (title) title.textContent = stream.title || 'Untitled Stream';
    if (date) date.textContent = new Date(parseInt(stream.starts) * 1000).toLocaleDateString();
    if (summary) summary.textContent = stream.summary || 'No description available';
    
    if (tags && stream.tags) {
        tags.innerHTML = stream.tags.map(tag => 
            `<span class="neon-border text-green-400 px-3 py-1 text-sm rounded font-mono">#${tag}</span>`
        ).join('');
    }
    
    // Load video
    if (video && stream.recording_url) {
        loadModalVideo(video, stream.recording_url);
    }
    
    if (modal) modal.classList.remove('hidden');
}

function loadModalVideo(video, streamUrl) {
    if (currentHls) {
        currentHls.destroy();
        currentHls = null;
    }
    
    if (Hls.isSupported()) {
        currentHls = new Hls();
        currentHls.loadSource(streamUrl);
        currentHls.attachMedia(video);
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
        video.src = streamUrl;
    }
}

function closeModal() {
    const modal = document.getElementById('streamModal');
    const video = document.getElementById('modalVideo');
    
    if (modal) modal.classList.add('hidden');
    if (video) video.pause();
    
    if (currentHls) {
        currentHls.destroy();
        currentHls = null;
    }
}

// Close modal on outside click
document.addEventListener('click', function(e) {
    const modal = document.getElementById('streamModal');
    if (e.target === modal) {
        closeModal();
    }
});

// Close modal on escape key
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        closeModal();
    }
});