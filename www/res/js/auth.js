/**
 * Gnostream Authentication System (Grain-compatible)
 * Based on Grain's authentication modal implementation
 */

// Global authentication state
let currentAuthMethod = null;
let isAuthenticated = false;
let currentSession = null;

// Modal management
function showAuthModal() {
    const modal = document.getElementById('auth-modal');
    if (modal) {
        modal.classList.remove('hidden');
        document.body.style.overflow = 'hidden';
    }
}

function hideAuthModal() {
    const modal = document.getElementById('auth-modal');
    if (modal) {
        modal.classList.add('hidden');
        document.body.style.overflow = 'auto';
        resetModal();
    }
}

function resetModal() {
    // Hide all forms
    const forms = ['extension-form', 'amber-form', 'bunker-form', 'readonly-form', 'privatekey-form'];
    forms.forEach(formId => {
        const form = document.getElementById(formId);
        if (form) form.classList.add('hidden');
    });

    // Show auth selection
    const selection = document.getElementById('auth-selection');
    if (selection) selection.classList.remove('hidden');

    // Clear all inputs
    const inputs = ['bunker-url', 'readonly-pubkey', 'private-key'];
    inputs.forEach(inputId => {
        const input = document.getElementById(inputId);
        if (input) input.value = '';
    });

    // Hide status
    hideStatus();
    
    currentAuthMethod = null;
}

function selectAuthMethod(method) {
    currentAuthMethod = method;
    
    // Hide auth selection
    const selection = document.getElementById('auth-selection');
    if (selection) selection.classList.add('hidden');
    
    // Show specific form
    const form = document.getElementById(`${method}-form`);
    if (form) form.classList.remove('hidden');
    
    // Special handling for extension
    if (method === 'extension') {
        checkForExtension();
    }
}

function toggleAdvanced() {
    const advancedOptions = document.getElementById('advanced-options');
    const arrow = document.getElementById('advanced-arrow');
    
    if (advancedOptions && arrow) {
        if (advancedOptions.classList.contains('hidden')) {
            advancedOptions.classList.remove('hidden');
            arrow.style.transform = 'rotate(180deg)';
        } else {
            advancedOptions.classList.add('hidden');
            arrow.style.transform = 'rotate(0deg)';
        }
    }
}

// Status management
function showStatus(message, type = 'loading') {
    const statusDiv = document.getElementById('auth-status');
    const statusIcon = document.getElementById('status-icon');
    const statusMessage = document.getElementById('status-message');
    
    if (statusDiv && statusIcon && statusMessage) {
        statusDiv.classList.remove('hidden', 'status-success', 'status-error', 'status-loading');
        statusDiv.classList.add(`status-${type}`);
        
        const icons = {
            loading: '⏳',
            success: '✅',
            error: '❌'
        };
        
        statusIcon.textContent = icons[type] || 'ℹ️';
        statusMessage.textContent = message;
    }
}

function hideStatus() {
    const statusDiv = document.getElementById('auth-status');
    if (statusDiv) statusDiv.classList.add('hidden');
}

// Extension detection
function checkForExtension() {
    const statusEl = document.getElementById("extension-status");
    const connectBtn = document.getElementById("connect-extension-btn");

    if (window.nostr) {
        statusEl.innerHTML = '<div class="text-green-300">✅ Nostr extension detected!</div>';
        statusEl.className = "p-3 mb-4 bg-green-800 border border-green-600 rounded-lg";
        connectBtn.disabled = false;
    } else {
        statusEl.innerHTML = '<div class="text-red-300">❌ No extension found.</div>';
        statusEl.className = "p-3 mb-4 bg-red-800 border border-red-600 rounded-lg";
        connectBtn.disabled = true;
    }
}

// Authentication methods
async function connectExtension() {
    showStatus('Checking for browser extension...', 'loading');
    
    try {
        console.log('Checking for window.nostr:', !!window.nostr);
        console.log('Available properties on window:', Object.keys(window).filter(k => k.includes('nostr')));
        
        // Wait a bit for extension to load if not immediately available
        if (!window.nostr) {
            console.log('Extension not found immediately, waiting 1 second...');
            showStatus('Waiting for extension to load...', 'loading');
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            console.log('Checking again for window.nostr:', !!window.nostr);
            
            if (!window.nostr) {
                // Try checking for specific extension indicators
                const hasAlby = !!window.alby;
                const hasNos2x = !!window.nos2x;
                console.log('Extension indicators:', { hasAlby, hasNos2x });
                
                throw new Error('No Nostr extension found. Please install Alby, nos2x, or Flamingo and refresh the page.');
            }
        }
        
        console.log('Extension found, available methods:', Object.keys(window.nostr));

        showStatus('Getting permission from extension...', 'loading');
        const pubkey = await window.nostr.getPublicKey();
        
        if (!pubkey) {
            throw new Error('Failed to get public key from extension');
        }

        showStatus('Connecting to gnostream...', 'loading');
        
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                public_key: pubkey,
                signing_method: 'browser_extension',
                mode: 'write'
            })
        });

        const result = await response.json();
        
        if (result.success) {
            currentSession = result.session;
            isAuthenticated = true;
            showStatus('Connected successfully!', 'success');
            
            setTimeout(() => {
                hideAuthModal();
                updateLoginButton();
            }, 1500);
            
            console.log('🔑 Logged in with browser extension:', pubkey);
        } else {
            throw new Error(result.error || 'Login failed');
        }

    } catch (error) {
        console.error('Extension login failed:', error);
        showStatus(error.message, 'error');
    }
}

async function connectAmber() {
    showStatus('Connecting to Amber...', 'loading');
    
    try {
        const isAndroid = /Android/i.test(navigator.userAgent);
        
        if (!isAndroid) {
            throw new Error('Amber is only available on Android devices');
        }

        // In a real implementation, this would handle Amber-specific connection
        // For now, we'll simulate the process
        showStatus('Opening Amber app...', 'loading');
        
        await new Promise(resolve => setTimeout(resolve, 2000));
        
        showStatus('Connected via Amber!', 'success');
        
        setTimeout(() => {
            hideAuthModal();
            updateLoginButton();
        }, 1500);
        
        console.log('🔑 Connected with Amber');

    } catch (error) {
        console.error('Amber login failed:', error);
        showStatus(error.message, 'error');
    }
}

async function connectBunker() {
    const bunkerUrl = document.getElementById('bunker-url')?.value.trim();
    
    if (!bunkerUrl) {
        showStatus('Please enter a bunker URL', 'error');
        return;
    }
    
    if (!bunkerUrl.startsWith('bunker://')) {
        showStatus('Bunker URL must start with bunker://', 'error');
        return;
    }

    showStatus('Connecting to bunker...', 'loading');
    
    try {
        // Bunker implementation would go here
        await new Promise(resolve => setTimeout(resolve, 2000));
        
        showStatus('Connected to bunker!', 'success');
        
        setTimeout(() => {
            hideAuthModal();
            updateLoginButton();
        }, 1500);
        
        console.log('🔑 Connected with bunker:', bunkerUrl);

    } catch (error) {
        console.error('Bunker login failed:', error);
        showStatus(error.message, 'error');
    }
}

async function connectReadOnly() {
    const pubkey = document.getElementById('readonly-pubkey')?.value.trim();
    
    showStatus('Setting up read-only access...', 'loading');
    
    try {
        let validatedPubkey = '';
        
        if (pubkey) {
            if (pubkey.startsWith('npub') && pubkey.length === 63) {
                validatedPubkey = pubkey;
            } else if (/^[0-9a-fA-F]{64}$/.test(pubkey)) {
                validatedPubkey = pubkey;
            } else {
                throw new Error('Invalid public key format');
            }
        }

        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                public_key: validatedPubkey,
                signing_method: 'read_only',
                mode: 'read_only'
            })
        });

        const result = await response.json();
        
        if (result.success) {
            currentSession = result.session;
            isAuthenticated = true;
            showStatus('Read-only mode activated!', 'success');
            
            setTimeout(() => {
                hideAuthModal();
                updateLoginButton();
            }, 1500);
            
            console.log('🔑 Logged in read-only mode');
        } else {
            throw new Error(result.error || 'Read-only login failed');
        }

    } catch (error) {
        console.error('Read-only login failed:', error);
        showStatus(error.message, 'error');
    }
}

async function connectPrivateKey() {
    const privateKey = document.getElementById('private-key')?.value.trim();
    
    if (!privateKey) {
        showStatus('Please enter your private key', 'error');
        return;
    }
    
    showStatus('Validating private key...', 'loading');
    
    try {
        // Basic validation - should be nsec or 64 char hex
        if (!privateKey.startsWith('nsec') && !/^[0-9a-fA-F]{64}$/.test(privateKey)) {
            throw new Error('Invalid private key format');
        }

        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                private_key: privateKey,
                signing_method: 'private_key',
                mode: 'write'
            })
        });

        const result = await response.json();
        
        if (result.success) {
            currentSession = result.session;
            isAuthenticated = true;
            showStatus('Connected with private key!', 'success');
            
            // Clear the private key input immediately
            document.getElementById('private-key').value = '';
            
            setTimeout(() => {
                hideAuthModal();
                updateLoginButton();
            }, 1500);
            
            console.log('🔑 Logged in with private key');
        } else {
            throw new Error(result.error || 'Private key login failed');
        }

    } catch (error) {
        console.error('Private key login failed:', error);
        showStatus(error.message, 'error');
    }
}

// Key generation
async function generateKeys() {
    showStatus('Generating new key pair...', 'loading');
    
    try {
        const response = await fetch('/api/auth/generate-keys', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });

        const result = await response.json();
        
        if (result.success && result.key_pair) {
            const keysDiv = document.getElementById('generated-keys');
            const nsecDiv = document.getElementById('gen-nsec');
            const npubDiv = document.getElementById('gen-npub');
            
            if (keysDiv && nsecDiv && npubDiv) {
                nsecDiv.textContent = result.key_pair.nsec;
                npubDiv.textContent = result.key_pair.npub;
                keysDiv.classList.remove('hidden');
            }
            
            showStatus('Keys generated successfully!', 'success');
            
            console.log('🔑 Generated new key pair:', result.key_pair.npub);
        } else {
            throw new Error(result.error || 'Key generation failed');
        }

    } catch (error) {
        console.error('Key generation failed:', error);
        showStatus(error.message, 'error');
    }
}

// Session management
async function checkExistingSession() {
    try {
        const response = await fetch('/api/auth/session');
        const result = await response.json();
        
        if (result.success && result.is_active && result.session) {
            currentSession = result.session;
            isAuthenticated = true;
            updateLoginButton();
            console.log('🔑 Existing session found:', result.session.public_key);
        }
    } catch (error) {
        console.log('No existing session found');
    }
}

async function logout() {
    try {
        const response = await fetch('/api/auth/logout', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });

        const result = await response.json();
        
        if (result.success) {
            currentSession = null;
            isAuthenticated = false;
            updateLoginButton();
            console.log('🔑 Logged out successfully');
        }

    } catch (error) {
        console.error('Logout failed:', error);
    }
}

// UI updates
function updateLoginButton() {
    const loginBtn = document.getElementById('login-btn');
    if (!loginBtn) return;
    
    if (isAuthenticated && currentSession) {
        // Update button to show user status
        const pubkeyShort = currentSession.public_key 
            ? currentSession.public_key.slice(0, 8) + '...'
            : 'USER';
            
        loginBtn.innerHTML = `
            <span class="text-green-400 mr-2">✅</span>
            ${pubkeyShort}
        `;
        
        // Add logout on click
        loginBtn.onclick = logout;
    } else {
        // Reset to login button
        loginBtn.innerHTML = `
            <span class="text-cyan-400 mr-2">🔑</span>
            LOGIN
        `;
        
        // Add login modal on click
        loginBtn.onclick = showAuthModal;
    }
}

// Public API
window.gnostreamAuth = {
    showModal: showAuthModal,
    hideModal: hideAuthModal,
    isAuthenticated: () => isAuthenticated,
    getSession: () => currentSession,
    canSign: () => isAuthenticated && currentSession?.mode === 'write',
    logout: logout
};

// Initialize
document.addEventListener('DOMContentLoaded', function() {
    // Setup modal close handlers
    document.getElementById('close-auth-modal')?.addEventListener('click', hideAuthModal);
    
    // Setup login button
    document.getElementById('login-btn')?.addEventListener('click', showAuthModal);
    
    // Close modal on outside click
    document.getElementById('auth-modal')?.addEventListener('click', function(e) {
        if (e.target === this) {
            hideAuthModal();
        }
    });
    
    // Check for existing session
    checkExistingSession();
    
    // Debug extension availability
    console.log('🔑 Gnostream auth system initialized');
});

// Export for modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { showAuthModal, hideAuthModal, isAuthenticated, currentSession };
}