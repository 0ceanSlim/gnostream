# Gnostream Nostr Login System

Gnostream now features a comprehensive Nostr authentication system powered by the Grain library. This enables users to connect with various signing methods and interact with the Nostr ecosystem directly through the streaming interface.

## Features

### ✅ **Working Login Methods**

#### **1. Browser Extension (NIP-07)**
- **Status**: ✅ FULLY IMPLEMENTED
- **Extensions Supported**: Alby, nos2x, Flamingo, and other NIP-07 compatible extensions
- **Capabilities**: Full read/write access, event signing, profile management

#### **2. Amber (Android External Signing)**
- **Status**: ✅ FULLY IMPLEMENTED 
- **Platform**: Android only
- **Capabilities**: External signing via Amber app, secure key management

#### **3. Read-Only Mode**
- **Status**: ✅ FULLY IMPLEMENTED
- **Usage**: Browse and view content without signing capabilities
- **Optional**: Can provide public key (npub/hex) for personalized experience

#### **4. Key Generation**
- **Status**: ✅ FULLY IMPLEMENTED
- **Features**: Generate secure Nostr key pairs directly in the interface
- **Output**: Provides both nsec (private) and npub (public) keys

### 🚧 **Placeholder Methods (Coming Soon)**

#### **5. Private Key Login**
- **Status**: 🚧 UI READY, BACKEND PENDING
- **Security**: Will include proper security warnings
- **Usage**: Direct nsec input (not recommended for production)

#### **6. Hardware Wallet Integration**
- **Status**: 🚧 PLANNED
- **Devices**: Ledger, Trezor support planned
- **Security**: Hardware-backed key security

#### **7. NIP-46 Remote Signing**
- **Status**: 🚧 PLANNED  
- **Protocol**: Remote signing bunker protocol
- **Usage**: bunker:// URLs for remote key management

## Frontend Implementation

### **Login Modal Interface**
The login system features a cyberpunk-themed modal with:

- **Visual Status Indicators**: ✅ Active, 🚧 WIP, 📋 Planned
- **Real-time Connection Status**: Loading, success, and error states
- **User Session Display**: Shows connected pubkey, mode, and signing method
- **Responsive Design**: Works on desktop and mobile devices

### **JavaScript API**

```javascript
// Access the global auth instance
const auth = window.gnostreamAuth;

// Check authentication status
if (auth.isAuthenticated()) {
    console.log('User is logged in:', auth.getSession());
}

// Check signing capabilities
if (auth.canSign()) {
    console.log('User can sign events');
}

// Show login modal programmatically
auth.showLoginModal();
```

### **Event Handling**
The system automatically:
- Checks for existing sessions on page load
- Handles browser extension permissions
- Manages session state across page navigation
- Provides visual feedback for all operations

## Backend API

### **Authentication Endpoints**

#### **POST `/api/auth/login`**
Authenticate users with various signing methods.

```json
{
    "public_key": "hex_or_npub",
    "private_key": "nsec_format", // Optional, for certain methods
    "signing_method": "browser_extension|amber|read_only",
    "mode": "read_only|write"
}
```

**Response:**
```json
{
    "success": true,
    "message": "Login successful",
    "session": {
        "public_key": "hex_pubkey",
        "mode": "write",
        "signing_method": "browser_extension",
        "last_active": "2025-09-09T17:00:00Z",
        "connected_relays": ["wss://relay1.com", "wss://relay2.com"]
    },
    "npub": "npub1..."
}
```

#### **GET `/api/auth/session`**
Check current session status.

**Response:**
```json
{
    "success": true,
    "is_active": true,
    "session": {
        "public_key": "hex_pubkey",
        "mode": "write",
        "signing_method": "browser_extension"
    }
}
```

#### **POST `/api/auth/logout`**
End current user session.

#### **POST `/api/auth/generate-keys`**
Generate new Nostr key pairs.

**Response:**
```json
{
    "success": true,
    "key_pair": {
        "private_key": "hex_private",
        "public_key": "hex_public", 
        "nsec": "nsec1...",
        "npub": "npub1..."
    }
}
```

#### **POST `/api/auth/connect-relay`**
Add new relay connections.

```json
{
    "relay_url": "wss://new-relay.com"
}
```

## Integration with Grain

The authentication system is built on top of the [Grain](https://github.com/0ceanslim/grain) Nostr library, providing:

- **Advanced Session Management**: Proper user session handling with security
- **Connection Pooling**: Efficient relay connection management
- **Event Building**: Fluent API for creating and signing Nostr events
- **Broadcast Analytics**: Success/failure tracking across relays
- **Key Management**: Secure key generation and conversion utilities

### **Session Types**

```go
// WriteMode - Full signing capabilities
userSession := &session.UserSession{
    Mode: session.WriteMode,
    SigningMethod: session.BrowserExtension,
}

// ReadOnlyMode - View-only access
userSession := &session.UserSession{
    Mode: session.ReadOnlyMode,
}
```

## Usage Examples

### **Basic Browser Extension Login**

```javascript
// User clicks login button
document.getElementById('login-btn').addEventListener('click', () => {
    window.gnostreamAuth.showLoginModal();
});

// After successful login
window.gnostreamAuth.currentSession = {
    public_key: "16f1a0100d4cfffbcc4230e8e0e4290cc5849c1adc64d6653fda07c031b1074b",
    mode: "write",
    signing_method: "browser_extension"
};
```

### **Checking User Permissions**

```javascript
const auth = window.gnostreamAuth;

// Before attempting to sign events
if (!auth.canSign()) {
    alert('You need to login with signing capabilities first');
    auth.showLoginModal();
    return;
}

// Proceed with signing
console.log('User can sign events');
```

### **Session Persistence**

The system automatically:
1. Saves session cookies for persistence
2. Checks for existing sessions on page load
3. Maintains relay connections across navigation
4. Handles session expiration gracefully

## Security Considerations

### **Current Implementation**
- ✅ **Browser Extension**: Secure, keys never leave extension
- ✅ **Amber**: External signing, keys protected by Android app
- ✅ **Read-Only**: No signing capabilities, safe for browsing
- ✅ **Generated Keys**: Client-side generation, user responsible for storage

### **Future Security Enhancements**
- 🚧 **Hardware Wallets**: Hardware-backed security
- 🚧 **NIP-46 Bunkers**: Remote signing with proper authentication
- 🚧 **Private Key Warning**: Enhanced security warnings and best practices
- 🚧 **Session Encryption**: Encrypted session storage

## Mobile Support

The login interface is fully responsive and includes:
- **Touch-friendly Controls**: Proper button sizing for mobile
- **Responsive Modal**: Adapts to screen size
- **Android Amber Support**: Native integration with Amber app
- **iOS Compatibility**: Works with iOS browser extensions

## Troubleshooting

### **Common Issues**

#### **"No NIP-07 extension found"**
- **Solution**: Install a compatible browser extension like Alby or nos2x
- **Check**: Extension is enabled and permissions granted

#### **"Failed to connect to relay"**
- **Solution**: Check relay URLs in configuration
- **Debug**: View browser console for connection errors

#### **"Session expired"**
- **Solution**: Login again through the modal
- **Prevention**: Sessions auto-refresh with activity

#### **Amber not responding**
- **Android Only**: Amber is an Android-specific app
- **Check**: Amber app is installed and up to date
- **Alternative**: Use browser extension on other platforms

## Development

### **Adding New Signing Methods**

1. **Update Frontend Modal** (`login-modal.html`):
```html
<div class="login-option border border-green-400 rounded p-4">
    <h3 class="text-green-400 font-mono">New Method</h3>
    <button id="login-newmethod">CONNECT_NEWMETHOD</button>
</div>
```

2. **Add JavaScript Handler** (`auth.js`):
```javascript
async loginWithNewMethod() {
    // Implementation here
}
```

3. **Update Backend API** (`auth.go`):
```go
case "new_method":
    // Handle new signing method
```

### **Testing Login Methods**

```javascript
// Test browser extension availability
console.log('NIP-07 available:', !!window.nostr);

// Test session state
console.log('Current session:', window.gnostreamAuth.getSession());

// Test API endpoints
fetch('/api/auth/session').then(r => r.json()).then(console.log);
```

## Future Roadmap

### **Phase 1 (Current)**
- ✅ Browser extension login
- ✅ Amber wallet integration  
- ✅ Read-only mode
- ✅ Key generation

### **Phase 2 (In Development)**
- 🚧 Private key login with security warnings
- 🚧 Enhanced session management
- 🚧 Relay management interface

### **Phase 3 (Planned)**
- 📋 Hardware wallet support (Ledger, Trezor)
- 📋 NIP-46 remote signing bunkers
- 📋 Multi-signature support
- 📋 Advanced permission management

The login system provides a solid foundation for Nostr integration while maintaining security and user experience as top priorities.