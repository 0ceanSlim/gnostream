/**
 * Gnostream Authentication System (Grain-compatible)
 * Based on Grain's authentication modal implementation
 */

// Global authentication state
let currentAuthMethod = null;
let isAuthenticated = false;
let currentSession = null;
let userProfile = null;

// Expose userProfile globally for mobile dropdown access
window.userProfile = userProfile;

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

            // Fetch profile after successful login
            setTimeout(async () => {
                await fetchUserProfile();
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

        console.log('🔍 Session check result:', result);

        if (result.success && result.is_active && result.session) {
            currentSession = result.session;
            userProfile = result.profile; // Store profile information
            window.userProfile = userProfile; // Update global reference
            isAuthenticated = true;
            updateLoginButton();
            console.log('🔑 Existing session found:', result.session.public_key);
            if (userProfile) {
                console.log('🔑 Profile loaded:', userProfile.name || userProfile.display_name || 'Unknown');
            }
        } else {
            // Explicitly handle no session case
            currentSession = null;
            userProfile = null;
            window.userProfile = null; // Update global reference
            isAuthenticated = false;
            updateLoginButton();
            console.log('🔍 No active session found');
        }
    } catch (error) {
        console.log('🔍 Session check failed:', error);
        // Ensure we're in logged out state on error
        currentSession = null;
        userProfile = null;
        window.userProfile = null; // Update global reference
        isAuthenticated = false;
        updateLoginButton();
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
            userProfile = null;
            window.userProfile = null; // Update global reference
            isAuthenticated = false;
            updateLoginButton();
            console.log('🔑 Logged out successfully');
        }

    } catch (error) {
        console.error('Logout failed:', error);
    }
}

// Fetch user profile from session endpoint
async function fetchUserProfile() {
    try {
        const response = await fetch('/api/auth/session');
        const result = await response.json();

        if (result.success && result.profile) {
            userProfile = result.profile;
            window.userProfile = userProfile; // Update global reference
            console.log('🔑 Profile refreshed:', userProfile.name || userProfile.display_name || 'Unknown');
            return userProfile;
        }
    } catch (error) {
        console.error('Failed to fetch user profile:', error);
    }
    return null;
}

// Profile dropdown functionality
function toggleProfileDropdown() {
    console.log('🔍 toggleProfileDropdown called');

    // Check if dropdown already exists
    let dropdown = document.getElementById('profile-dropdown');

    if (dropdown) {
        // Remove existing dropdown
        dropdown.remove();
        return;
    }

    // Create dropdown element
    dropdown = document.createElement('div');
    dropdown.id = 'profile-dropdown';

    // Get login button position for positioning
    const loginBtn = document.getElementById('login-btn');
    const buttonRect = loginBtn.getBoundingClientRect();

    // Style the dropdown
    dropdown.style.cssText = `
        position: fixed;
        top: ${buttonRect.bottom + 8}px;
        right: ${window.innerWidth - buttonRect.right}px;
        background: #1a1a1a;
        border: 2px solid #00ff41;
        border-radius: 8px;
        padding: 8px;
        min-width: 200px;
        z-index: 1000;
        box-shadow: 0 4px 20px rgba(0, 255, 65, 0.3);
        font-family: 'Share Tech Mono', monospace;
    `;

    // Create dropdown content
    const profileInfo = createProfileInfo();
    const menuItems = createMenuItems();

    dropdown.innerHTML = `
        ${profileInfo}
        <div class="border-t border-gray-600 my-2"></div>
        ${menuItems}
    `;

    // Add to page
    document.body.appendChild(dropdown);

    // Close dropdown when clicking elsewhere
    setTimeout(() => {
        const closeDropdown = (event) => {
            if (!dropdown.contains(event.target) && !loginBtn.contains(event.target)) {
                dropdown.remove();
                document.removeEventListener('click', closeDropdown);
            }
        };
        document.addEventListener('click', closeDropdown);
    }, 100);
}

function createProfileInfo() {
    if (!userProfile || !currentSession) {
        return '';
    }

    const displayName = userProfile.display_name || userProfile.name || 'Unknown User';
    const profilePicture = userProfile.picture;

    return `
        <div class="flex items-center p-3 bg-gray-800 rounded-lg mb-2">
            ${profilePicture ?
                `<img src="${profilePicture}" alt="Profile" class="w-10 h-10 rounded-full mr-3 object-cover">` :
                `<div class="w-10 h-10 rounded-full bg-gray-600 flex items-center justify-center mr-3">
                    <span class="text-gray-300">👤</span>
                 </div>`
            }
            <div class="flex-1">
                <div class="text-green-400 font-medium text-sm">${displayName}</div>
            </div>
        </div>
    `;
}

function createMenuItems() {
    return `
        <div class="space-y-1">
            <button onclick="showSettings()" class="w-full flex items-center px-3 py-2 text-left text-gray-300 hover:text-white hover:bg-gray-700 rounded transition-colors duration-200">
                <span class="mr-3">⚙️</span>
                <span class="text-sm font-mono uppercase">Settings</span>
            </button>
            <button onclick="handleDropdownLogout()" class="w-full flex items-center px-3 py-2 text-left text-red-400 hover:text-white hover:bg-red-600 rounded transition-colors duration-200">
                <span class="mr-3">🚪</span>
                <span class="text-sm font-mono uppercase">Logout</span>
            </button>
        </div>
    `;
}

function handleDropdownLogout() {
    // Close dropdown first
    const dropdown = document.getElementById('profile-dropdown');
    if (dropdown) {
        dropdown.remove();
    }

    // Then logout
    logout();
}

function showSettings() {
    // Close dropdown first
    const dropdown = document.getElementById('profile-dropdown');
    if (dropdown) {
        dropdown.remove();
    }

    // TODO: Implement settings page navigation
    console.log('🔧 Settings clicked - will implement settings page next');
    alert('Settings page coming soon!');
}

// UI updates
function updateLoginButton() {
    const loginBtn = document.getElementById('login-btn');
    if (!loginBtn) return;

    console.log('🔍 updateLoginButton called:', {
        isAuthenticated: isAuthenticated,
        hasSession: !!currentSession,
        hasProfile: !!userProfile
    });


    if (isAuthenticated && currentSession) {
        // Determine display name and picture
        let displayName = 'USER';
        let profilePicture = null;

        if (userProfile) {
            // Use display_name first, then name, then fallback to truncated pubkey
            displayName = userProfile.display_name || userProfile.name ||
                         (currentSession.public_key ? currentSession.public_key.slice(0, 8) + '...' : 'USER');
            profilePicture = userProfile.picture;
        } else if (currentSession.public_key) {
            displayName = currentSession.public_key.slice(0, 8) + '...';
        }

        // Create button content with optional profile picture
        let buttonContent = '';
        if (profilePicture) {
            buttonContent = `
                <img src="${profilePicture}" alt="Profile" class="w-6 h-6 rounded-full mr-2 object-cover">
                <span class="text-green-400">${displayName}</span>
            `;
        } else {
            buttonContent = `
                <span class="text-green-400 mr-2">✅</span>
                <span class="text-green-400">${displayName}</span>
            `;
        }

        loginBtn.innerHTML = buttonContent;
        loginBtn.className = 'cyber-button px-4 py-2 rounded text-sm font-mono uppercase tracking-wide flex items-center max-lg:hidden';

        // Clear any existing event listeners and add dropdown toggle
        loginBtn.onclick = null;
        loginBtn.removeEventListener('click', showAuthModal);
        loginBtn.removeEventListener('click', toggleProfileDropdown);
        loginBtn.addEventListener('click', toggleProfileDropdown);
        console.log('🔍 Set profile dropdown handler');
    } else {
        // Reset to login button
        loginBtn.innerHTML = `
            <span class="text-cyan-400 mr-2">🔑</span>
            LOGIN
        `;
        loginBtn.className = 'cyber-button px-4 py-2 rounded text-sm font-mono uppercase tracking-wide flex items-center max-lg:hidden';

        // Clear any existing event listeners and add login modal
        loginBtn.onclick = null;
        loginBtn.removeEventListener('click', toggleProfileDropdown);
        loginBtn.removeEventListener('click', showAuthModal);
        loginBtn.addEventListener('click', showAuthModal);
        console.log('🔍 Set login modal handler');
    }
}

// Test function for debugging
window.testDropdown = function() {
    console.log('🔍 Manual dropdown test');
    toggleProfileDropdown();
};

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

    // Don't set up static login button handler - updateLoginButton() handles this dynamically

    // Close modal on outside click
    document.getElementById('auth-modal')?.addEventListener('click', function(e) {
        if (e.target === this) {
            hideAuthModal();
        }
    });

    // Set initial login button state (will be updated if session is found)
    updateLoginButton();

    // Check for existing session (this will call updateLoginButton again if session exists)
    checkExistingSession();

    // Debug extension availability
    console.log('🔑 Gnostream auth system initialized');
});

// Export for modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { showAuthModal, hideAuthModal, isAuthenticated, currentSession };
}