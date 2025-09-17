/**
 * Gnostream Authentication System (Grain-compatible)
 * Based on Grain's authentication modal implementation
 */

// Global authentication state
let currentAuthMethod = null;
let isAuthenticated = false;
let currentSession = null;
let userProfile = null;
let encryptedPrivateKey = null;
let privateKeyPassword = null;

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
    const inputs = ['bunker-url', 'readonly-pubkey', 'private-key', 'private-key-password'];
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

let amberCallbackReceived = false;

async function connectAmber() {
    showStatus('Connecting to Amber...', 'loading');

    try {
        const isAndroid = /Android/i.test(navigator.userAgent);

        if (!isAndroid) {
            throw new Error('Amber is only available on Android devices');
        }

        showStatus('Opening Amber app...', 'loading');

        // Set up callback listener BEFORE opening Amber
        setupAmberCallbackListener();

        // Generate proper callback URL for gnostream
        const callbackUrl = `${window.location.origin}/api/auth/amber-callback?event=`;

        // Use proper NIP-55 nostrsigner URL format
        const amberUrl = `nostrsigner:?compressionType=none&returnType=signature&type=get_public_key&callbackUrl=${encodeURIComponent(callbackUrl)}&appName=${encodeURIComponent("gnostream")}`;

        console.log('🔍 Opening Amber with URL:', amberUrl);

        // Try multiple approaches for opening the nostrsigner protocol
        let protocolOpened = false;

        // Method 1: Create anchor element and click it (most reliable on mobile)
        try {
            const anchor = document.createElement("a");
            anchor.href = amberUrl;
            anchor.target = "_blank";
            anchor.style.display = "none";
            document.body.appendChild(anchor);

            anchor.click();
            protocolOpened = true;

            setTimeout(() => {
                if (document.body.contains(anchor)) {
                    document.body.removeChild(anchor);
                }
            }, 100);

            console.log('🔍 Amber protocol opened via anchor click');
        } catch (anchorError) {
            console.warn('⚠️ Anchor method failed:', anchorError);
        }

        // Method 2: Fallback to window.location.href if anchor didn't work
        if (!protocolOpened) {
            try {
                window.location.href = amberUrl;
                protocolOpened = true;
                console.log('🔍 Amber protocol opened via window.location.href');
            } catch (locationError) {
                console.warn('⚠️ Window location method failed:', locationError);
            }
        }

        if (!protocolOpened) {
            throw new Error('Unable to open Amber protocol - make sure Amber is installed');
        }

        showStatus('Opening Amber app... If nothing happens, make sure Amber is installed and try again.', 'loading');

        // Set timeout in case user doesn't complete the flow
        setTimeout(() => {
            if (!amberCallbackReceived) {
                showStatus('Amber connection timed out. Make sure Amber is installed and try again.', 'error');
            }
        }, 60000); // 60 seconds timeout

    } catch (error) {
        console.error('❌ Amber connection failed:', error);
        showStatus(error.message, 'error');
    }
}

// Set up proper callback listener using window focus and URL checking
function setupAmberCallbackListener() {
    const handleVisibilityChange = () => {
        if (!document.hidden && !amberCallbackReceived) {
            setTimeout(checkForAmberCallback, 500);
        }
    };

    const handleFocus = () => {
        if (!amberCallbackReceived) {
            setTimeout(checkForAmberCallback, 500);
        }
    };

    // Add multiple listeners to catch the return
    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("focus", handleFocus);

    // Also check immediately
    setTimeout(checkForAmberCallback, 1000);

    // Clean up listeners after timeout
    setTimeout(() => {
        document.removeEventListener("visibilitychange", handleVisibilityChange);
        window.removeEventListener("focus", handleFocus);
    }, 65000);
}

// Check if we're on the callback URL or if callback data is available
function checkForAmberCallback() {
    const currentUrl = new URL(window.location.href);

    // Check if this is the amber-callback page
    if (currentUrl.pathname === "/api/auth/amber-callback") {
        handleAmberCallback(currentUrl);
        return;
    }

    // Check if we have the event parameter in current URL
    if (currentUrl.searchParams.has("event")) {
        handleAmberCallback(currentUrl);
        return;
    }

    // Check if URL has amber_login=success parameter
    if (currentUrl.searchParams.get("amber_login") === "success") {
        handleAmberSuccess();
        return;
    }

    // Check if data was stored in localStorage by the callback page
    const amberResult = localStorage.getItem("amber_callback_result");
    if (amberResult) {
        try {
            const data = JSON.parse(amberResult);
            localStorage.removeItem("amber_callback_result");
            handleAmberCallbackData(data);
        } catch (error) {
            console.error('❌ Failed to parse stored Amber result:', error);
        }
    }
}

// Handle callback from Amber with public key
function handleAmberCallback(url) {
    try {
        amberCallbackReceived = true;
        const eventParam = url.searchParams.get("event");

        if (!eventParam) {
            throw new Error("No event data received from Amber");
        }

        console.log('✅ Received Amber callback:', eventParam);
        handleAmberCallbackData({ event: eventParam });
    } catch (error) {
        console.error('❌ Error handling Amber callback:', error);
        showStatus(`Amber callback error: ${error.message}`, 'error');
    }
}

// Process the actual callback data
function handleAmberCallbackData(data) {
    try {
        if (data.error) {
            throw new Error(data.error);
        }

        amberCallbackReceived = true;
        console.log('✅ Amber login completed successfully');

        showStatus('Connected via Amber!', 'success');

        // Fetch profile and update UI
        setTimeout(async () => {
            await checkExistingSession(); // This will refresh session and profile
            hideAuthModal();
            updateLoginButton();
        }, 1000);

    } catch (error) {
        console.error('❌ Error processing Amber callback data:', error);
        showStatus(`Amber login failed: ${error.message}`, 'error');
    }
}

// Handle Amber success from URL parameter
function handleAmberSuccess() {
    amberCallbackReceived = true;
    console.log('✅ Amber login success detected from URL');

    showStatus('Connected via Amber!', 'success');

    setTimeout(async () => {
        await checkExistingSession();
        hideAuthModal();
        updateLoginButton();

        // Clean up URL parameter
        const url = new URL(window.location);
        url.searchParams.delete('amber_login');
        window.history.replaceState({}, document.title, url.toString());
    }, 1000);
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
    const password = document.getElementById('private-key-password')?.value.trim();

    if (!privateKey) {
        showStatus('Please enter your private key', 'error');
        return;
    }

    if (!password) {
        showStatus('Please enter a password to encrypt your private key', 'error');
        return;
    }

    if (password.length < 8) {
        showStatus('Password must be at least 8 characters long', 'error');
        return;
    }

    showStatus('Validating private key...', 'loading');

    try {
        // Basic validation - should be nsec or 64 char hex
        if (!privateKey.startsWith('nsec') && !/^[0-9a-fA-F]{64}$/.test(privateKey)) {
            throw new Error('Invalid private key format');
        }

        // The backend expects nsec format or hex, and will derive the public key
        // No need to validate specific format here, let backend handle it

        showStatus('Encrypting private key...', 'loading');

        // Encrypt the private key with the password
        const encrypted = await encryptPrivateKey(privateKey, password);

        // Store encrypted key
        if (!storeEncryptedPrivateKey(encrypted)) {
            throw new Error('Failed to store encrypted private key');
        }

        // Set global variables for later use
        encryptedPrivateKey = encrypted;
        privateKeyPassword = password;

        showStatus('Authenticating...', 'loading');

        // Send login request with just the public key (derived from private key on backend)
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                private_key: privateKey, // Backend will derive public key and create session
                signing_method: 'private_key',
                mode: 'write'
            })
        });

        const result = await response.json();

        if (result.success) {
            currentSession = result.session;
            isAuthenticated = true;
            showStatus('Connected with private key!', 'success');

            // Clear the inputs immediately for security
            document.getElementById('private-key').value = '';
            document.getElementById('private-key-password').value = '';

            // Fetch profile after successful login
            setTimeout(async () => {
                await fetchUserProfile();
                hideAuthModal();
                updateLoginButton();
            }, 1500);

            console.log('🔑 Logged in with private key, encrypted and stored securely');
        } else {
            // Clear stored data on login failure
            clearStoredPrivateKey();
            throw new Error(result.error || 'Private key login failed');
        }

    } catch (error) {
        console.error('❌ Private key login failed:', error);
        showStatus(error.message, 'error');
        // Clear sensitive data on error
        clearStoredPrivateKey();
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

            // Clear stored private key on logout
            clearStoredPrivateKey();

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

// Private key encryption/decryption functions
async function encryptPrivateKey(privateKey, password) {
    const encoder = new TextEncoder();
    const keyMaterial = await crypto.subtle.importKey(
        'raw',
        encoder.encode(password),
        { name: 'PBKDF2' },
        false,
        ['deriveBits', 'deriveKey']
    );

    const salt = crypto.getRandomValues(new Uint8Array(16));
    const key = await crypto.subtle.deriveKey(
        {
            name: 'PBKDF2',
            salt: salt,
            iterations: 100000,
            hash: 'SHA-256'
        },
        keyMaterial,
        { name: 'AES-GCM', length: 256 },
        false,
        ['encrypt']
    );

    const iv = crypto.getRandomValues(new Uint8Array(12));
    const encrypted = await crypto.subtle.encrypt(
        { name: 'AES-GCM', iv: iv },
        key,
        encoder.encode(privateKey)
    );

    // Combine salt, iv, and encrypted data
    const combined = new Uint8Array(salt.length + iv.length + encrypted.byteLength);
    combined.set(salt);
    combined.set(iv, salt.length);
    combined.set(new Uint8Array(encrypted), salt.length + iv.length);

    return btoa(String.fromCharCode(...combined));
}

async function decryptPrivateKey(encryptedData, password) {
    const decoder = new TextDecoder();
    const encoder = new TextEncoder();

    // Decode from base64
    const combined = new Uint8Array(atob(encryptedData).split('').map(c => c.charCodeAt(0)));

    // Extract salt, iv, and encrypted data
    const salt = combined.slice(0, 16);
    const iv = combined.slice(16, 28);
    const encrypted = combined.slice(28);

    const keyMaterial = await crypto.subtle.importKey(
        'raw',
        encoder.encode(password),
        { name: 'PBKDF2' },
        false,
        ['deriveBits', 'deriveKey']
    );

    const key = await crypto.subtle.deriveKey(
        {
            name: 'PBKDF2',
            salt: salt,
            iterations: 100000,
            hash: 'SHA-256'
        },
        keyMaterial,
        { name: 'AES-GCM', length: 256 },
        false,
        ['decrypt']
    );

    const decrypted = await crypto.subtle.decrypt(
        { name: 'AES-GCM', iv: iv },
        key,
        encrypted
    );

    return decoder.decode(decrypted);
}

// Store encrypted private key in secure storage
function storeEncryptedPrivateKey(encryptedKey) {
    try {
        localStorage.setItem('gnostream_encrypted_key', encryptedKey);
        return true;
    } catch (error) {
        console.error('Failed to store encrypted private key:', error);
        return false;
    }
}

// Retrieve encrypted private key from storage
function getStoredEncryptedPrivateKey() {
    try {
        return localStorage.getItem('gnostream_encrypted_key');
    } catch (error) {
        console.error('Failed to retrieve encrypted private key:', error);
        return null;
    }
}

// Clear stored private key
function clearStoredPrivateKey() {
    try {
        localStorage.removeItem('gnostream_encrypted_key');
        encryptedPrivateKey = null;
        privateKeyPassword = null;
        return true;
    } catch (error) {
        console.error('Failed to clear stored private key:', error);
        return false;
    }
}

// Event signing with stored private key
async function signEventWithPrivateKey(eventObj, password = null) {
    try {
        // Use stored password if available, otherwise require password parameter
        const pwd = password || privateKeyPassword;
        if (!pwd) {
            throw new Error('Password required to decrypt private key');
        }

        // Get stored encrypted private key
        const storedKey = encryptedPrivateKey || getStoredEncryptedPrivateKey();
        if (!storedKey) {
            throw new Error('No stored private key found');
        }

        // Decrypt the private key
        const privateKey = await decryptPrivateKey(storedKey, pwd);

        // TODO: Implement actual event signing with nostr library
        // For now, this is a placeholder that would integrate with a nostr signing library
        console.log('🔐 Signing event with private key (placeholder)');
        console.log('Event to sign:', eventObj);

        // This would return the signed event
        return {
            ...eventObj,
            sig: 'placeholder_signature_' + Date.now()
        };

    } catch (error) {
        console.error('❌ Failed to sign event with private key:', error);
        throw error;
    }
}

// Check if private key signing is available
function canSignWithPrivateKey() {
    return !!(encryptedPrivateKey || getStoredEncryptedPrivateKey()) && !!privateKeyPassword;
}

// Prompt for password if needed for signing
async function promptForSigningPassword() {
    return new Promise((resolve, reject) => {
        const password = prompt('Enter your private key password to sign this event:');
        if (password) {
            resolve(password);
        } else {
            reject(new Error('Password required for signing'));
        }
    });
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
    logout: logout,
    signEvent: signEventWithPrivateKey,
    canSignWithPrivateKey: canSignWithPrivateKey,
    promptForPassword: promptForSigningPassword
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

    // Check for Amber callback on page load
    setTimeout(checkForAmberCallback, 500);

    // Debug extension availability
    console.log('🔑 Gnostream auth system initialized');
});

// Export for modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { showAuthModal, hideAuthModal, isAuthenticated, currentSession };
}