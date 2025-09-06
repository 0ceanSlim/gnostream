// Widget JavaScript for OBS Browser Sources

// Enhanced Nostr-specific user agent categorization
function categorizeUserAgent(ua) {
    const lower = ua.toLowerCase();
    
    // Specific Nostr clients
    if (lower.includes('amethyst')) {
        return { category: 'nostr', client: 'Amethyst', platform: 'Android' };
    }
    if (lower.includes('dalvik')) {
        return { category: 'nostr', client: 'Primal', platform: 'Android' };
    }
    if (ua.startsWith('Mozilla/5.0')) {
        // Determine platform for Nostrudel
        let platform = 'Web';
        if (lower.includes('windows')) platform = 'Windows';
        else if (lower.includes('macintosh') || lower.includes('mac os')) platform = 'macOS';
        else if (lower.includes('linux')) platform = 'Linux';
        else if (lower.includes('android')) platform = 'Android';
        else if (lower.includes('iphone') || lower.includes('ipad')) platform = 'iOS';
        
        return { category: 'nostr', client: 'Nostrudel', platform: platform };
    }
    
    // Native iOS media players (could be Damus or other Nostr clients)
    if (lower.includes('applecoremedia')) {
        const device = ua.includes('iPhone') ? 'iPhone' : (ua.includes('iPad') ? 'iPad' : 'iOS');
        return { category: 'native', client: 'iOS Media Player', platform: device };
    }
    
    // Regular browsers
    if (lower.includes('chrome') || lower.includes('firefox') || 
        lower.includes('safari') || lower.includes('edge') || lower.includes('opera')) {
        let browser = 'Browser';
        if (lower.includes('chrome')) browser = 'Chrome';
        else if (lower.includes('firefox')) browser = 'Firefox';
        else if (lower.includes('safari')) browser = 'Safari';
        else if (lower.includes('edge')) browser = 'Edge';
        else if (lower.includes('opera')) browser = 'Opera';
        
        let platform = 'Desktop';
        if (lower.includes('android')) platform = 'Android';
        else if (lower.includes('iphone') || lower.includes('ipad')) platform = 'iOS';
        
        return { category: 'browser', client: browser, platform: platform };
    }
    
    return { category: 'unknown', client: 'Unknown', platform: 'Unknown' };
}

// Update viewer count
function updateViewerCount() {
    fetch('/api/viewers')
        .then(response => response.json())
        .then(data => {
            const countEl = document.getElementById('viewer-count');
            if (countEl) {
                countEl.textContent = data.active_viewers || 0;
            }
        })
        .catch(error => {
            console.error('Error fetching viewer count:', error);
        });
}

// Update all-time viewer stats
function updateAllTimeStats() {
    fetch('/api/viewers')
        .then(response => response.json())
        .then(data => {
            const activeEl = document.getElementById('active');
            const peakEl = document.getElementById('peak');
            const totalEl = document.getElementById('total');
            
            if (activeEl) activeEl.textContent = data.active_viewers || 0;
            if (peakEl) peakEl.textContent = data.peak_viewers || 0;
            if (totalEl) totalEl.textContent = data.total_viewers || 0;
        })
        .catch(error => {
            console.error('Error fetching all-time stats:', error);
        });
}

// Update Nostr client breakdown (current viewers)
function updateNostrClientBreakdown() {
    fetch('/api/viewers')
        .then(response => response.json())
        .then(data => {
            const clients = {};
            const total = data.sessions ? data.sessions.length : 0;
            
            // Categorize each current session by specific client
            if (data.sessions) {
                data.sessions.forEach(session => {
                    const info = categorizeUserAgent(session.user_agent || '');
                    const key = `${info.client} (${info.platform})`;
                    
                    if (!clients[key]) {
                        clients[key] = { count: 0, category: info.category };
                    }
                    clients[key].count++;
                });
            }
            
            // Sort by count
            const sortedClients = Object.entries(clients)
                .sort((a, b) => b[1].count - a[1].count);
            
            let html = '';
            sortedClients.forEach(([clientName, info]) => {
                const count = info.count;
                const percentage = total > 0 ? Math.round((count / total) * 100) : 0;
                
                // Color coding by category
                let color = 'bg-gradient-to-r from-gray-500 to-gray-700'; // unknown
                if (info.category === 'nostr') color = 'bg-gradient-to-r from-purple-500 to-purple-700';
                else if (info.category === 'browser') color = 'bg-gradient-to-r from-orange-500 to-orange-700';
                else if (info.category === 'native') color = 'bg-gradient-to-r from-green-500 to-green-700';
                
                // Special highlighting for Nostr clients
                const textColor = info.category === 'nostr' ? 'text-purple-300' : 'text-green-300';
                
                html += `
                    <div class="client-item mb-3">
                        <div class="flex justify-between text-sm mb-1">
                            <span class="font-medium ${textColor}">${clientName}</span>
                            <span class="opacity-75 text-green-400">${count} (${percentage}%)</span>
                        </div>
                        <div class="w-full bg-gray-800 rounded-full h-3 overflow-hidden border border-green-900">
                            <div class="${color} h-full transition-all duration-700" style="width: ${percentage}%"></div>
                        </div>
                    </div>
                `;
            });
            
            if (html === '') {
                html = '<div class="text-center text-sm opacity-60">No active viewers</div>';
            }
            
            const breakdownEl = document.getElementById('breakdown-content');
            if (breakdownEl) {
                breakdownEl.innerHTML = html;
            }
        })
        .catch(error => {
            console.error('Error fetching client breakdown:', error);
        });
}

// Legacy function name for backward compatibility
const updateCategoryBreakdown = updateNostrClientBreakdown;

// Initialize widgets based on what's present on the page
document.addEventListener('DOMContentLoaded', function() {
    // Viewer count widget
    if (document.getElementById('viewer-count')) {
        updateViewerCount();
        setInterval(updateViewerCount, 5000);
    }
    
    // All-time viewer stats widget  
    if (document.getElementById('active')) {
        updateAllTimeStats();
        setInterval(updateAllTimeStats, 5000);
    }
    
    // Current viewers by Nostr client widget
    if (document.getElementById('breakdown-content')) {
        updateNostrClientBreakdown();
        setInterval(updateNostrClientBreakdown, 8000);
    }
});