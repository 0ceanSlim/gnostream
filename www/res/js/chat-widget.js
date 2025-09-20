/**
 * Live Chat Widget for Gnostream
 * Handles real-time nostr live chat (kind 1311) for the current stream
 */

class LiveChatWidget {
    constructor() {
        this.messages = [];
        this.isAuthenticated = false;
        this.userSession = null;
        this.currentReplyTo = null;
        this.ws = null;
        this.wsReconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.retryCount = 0;
        this.maxRetries = 5;
        this.isLoading = false;

        this.init();
    }

    init() {
        console.log('🔥 Initializing Live Chat Widget');

        this.setupEventListeners();
        this.checkAuthStatus();
        this.loadMessages();
        this.connectWebSocket();
    }

    setupEventListeners() {
        const messageInput = document.getElementById('message-input');
        const sendButton = document.getElementById('send-button');

        // Send message on button click
        sendButton?.addEventListener('click', () => this.sendMessage());

        // Send message on Enter key
        messageInput?.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendMessage();
            }
        });

        // Update character count
        messageInput?.addEventListener('input', () => this.updateCharCount());

        // Listen for auth changes from parent window
        window.addEventListener('message', (event) => {
            if (event.data.type === 'auth_changed') {
                console.log('🔑 Auth status changed, rechecking...');
                this.checkAuthStatus();
            }
        });

        // Check for auth changes periodically
        setInterval(() => this.checkAuthStatus(), 5000);
    }

    async checkAuthStatus() {
        try {
            const response = await fetch('/api/auth/session');
            const result = await response.json();

            const wasAuthenticated = this.isAuthenticated;
            this.isAuthenticated = result.success && result.is_active;
            this.userSession = result.session;

            if (wasAuthenticated !== this.isAuthenticated) {
                console.log(`🔑 Auth status changed: ${this.isAuthenticated ? 'logged in' : 'logged out'}`);
                this.updateInputUI();
            }

        } catch (error) {
            console.error('❌ Failed to check auth status:', error);
            this.isAuthenticated = false;
            this.userSession = null;
            this.updateInputUI();
        }
    }

    updateInputUI() {
        const loginPrompt = document.getElementById('login-prompt');
        const inputForm = document.getElementById('chat-input-form');

        if (this.isAuthenticated && this.userSession?.mode === 'write') {
            loginPrompt?.classList.add('hidden');
            inputForm?.classList.remove('hidden');
            this.updateSendButton();
        } else {
            loginPrompt?.classList.remove('hidden');
            inputForm?.classList.add('hidden');
        }
    }

    updateCharCount() {
        const messageInput = document.getElementById('message-input');
        const charCount = document.getElementById('char-count');

        if (messageInput && charCount) {
            const length = messageInput.value.length;
            charCount.textContent = `${length}/280`;

            if (length > 280) {
                charCount.style.color = '#ff4444';
            } else if (length > 240) {
                charCount.style.color = '#ffff00';
            } else {
                charCount.style.color = '#888';
            }
        }

        this.updateSendButton();
    }

    updateSendButton() {
        const messageInput = document.getElementById('message-input');
        const sendButton = document.getElementById('send-button');

        if (messageInput && sendButton) {
            const hasContent = messageInput.value.trim().length > 0;
            const validLength = messageInput.value.length <= 280;
            sendButton.disabled = !hasContent || !validLength || !this.isAuthenticated;
        }
    }

    async loadMessages() {
        // Prevent multiple simultaneous requests
        if (this.isLoading) {
            console.log('📝 Already loading messages, skipping...');
            return;
        }

        try {
            this.isLoading = true;
            console.log('📝 Loading chat messages...');
            this.showLoading('Loading messages...');

            const response = await fetch('/api/chat/messages');
            const result = await response.json();

            if (result.success) {
                this.messages = result.messages || [];

                // Clear any loading messages first
                this.clearLoadingMessages();

                this.renderMessages();
                this.retryCount = 0; // Reset retry count on success
                console.log(`📝 Loaded ${this.messages.length} chat messages`);
            } else {
                throw new Error(result.error || 'Failed to load messages');
            }

        } catch (error) {
            console.error('❌ Failed to load messages:', error);
            this.showError('Failed to load messages. Retrying...');
            this.retryCount++;

            if (this.retryCount < this.maxRetries) {
                setTimeout(() => this.loadMessages(), 2000 * this.retryCount);
            } else {
                this.showError('Unable to connect to chat. Please refresh the page.');
            }
        } finally {
            this.isLoading = false;
        }
    }

    renderMessages() {
        const messagesContainer = document.getElementById('chat-messages');
        const messageCountEl = document.getElementById('message-count');

        if (!messagesContainer) return;

        if (this.messages.length === 0) {
            // Only show "no messages" if there's nothing in the container
            if (messagesContainer.children.length === 0 ||
                messagesContainer.querySelector('#loading-indicator')) {
                messagesContainer.innerHTML = `
                    <div class="text-center text-gray-500 text-sm" id="no-messages">
                        <div>💬 No messages yet</div>
                        <div class="text-xs mt-1 opacity-75">Be the first to say something!</div>
                    </div>
                `;
            }
        } else {
            // Remove "no messages" indicator if it exists
            const noMessagesIndicator = messagesContainer.querySelector('#no-messages');
            if (noMessagesIndicator) {
                noMessagesIndicator.remove();
            }

            // Only add new messages, don't re-render everything
            this.addNewMessages(messagesContainer);
        }

        // Update message count
        if (messageCountEl) {
            const count = this.messages.length;
            messageCountEl.textContent = `${count} message${count !== 1 ? 's' : ''}`;
        }

        // Auto-scroll to bottom only if user is near bottom
        this.smartScrollToBottom();
    }

    addNewMessages(container) {
        // Clear any loading indicators
        const loadingIndicator = container.querySelector('#loading-indicator');
        if (loadingIndicator) {
            loadingIndicator.remove();
        }

        // Also clear any "Connecting to chat..." messages
        const connectingMessages = container.querySelectorAll('.loading-message');
        connectingMessages.forEach(msg => {
            if (msg.textContent.includes('Connecting') || msg.textContent.includes('Loading')) {
                const parent = msg.closest('.text-center');
                if (parent) {
                    parent.remove();
                }
            }
        });

        // Get existing message IDs
        const existingIds = new Set();
        container.querySelectorAll('[data-message-id]').forEach(el => {
            existingIds.add(el.getAttribute('data-message-id'));
        });

        // Track if we're adding any new messages
        let addedNewMessages = false;
        let newMessageCount = 0;

        // Add only new messages
        this.messages.forEach(message => {
            if (!existingIds.has(message.id)) {
                const messageEl = this.createMessageElement(message);
                messageEl.setAttribute('data-message-id', message.id);
                container.appendChild(messageEl);
                addedNewMessages = true;
                newMessageCount++;
            }
        });

        // Log only if we actually added new messages
        if (addedNewMessages) {
            console.log(`💬 Added ${newMessageCount} new messages to chat`);
        }
    }

    smartScrollToBottom() {
        const messagesContainer = document.getElementById('chat-messages');
        if (!messagesContainer) return;

        // Check if user is near the bottom (within 100px)
        const isNearBottom = messagesContainer.scrollTop + messagesContainer.clientHeight >=
                           messagesContainer.scrollHeight - 100;

        if (isNearBottom) {
            this.scrollToBottom();
        }
    }

    createMessageElement(message) {
        const messageDiv = document.createElement('div');
        messageDiv.className = 'chat-message';

        // Format timestamp
        const timestamp = new Date(message.created_at * 1000);
        const timeStr = timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

        // Get user display info
        const displayName = this.getUserDisplayName(message);
        const profilePic = message.profile?.picture;

        // Check if this is a reply
        const isReply = message.reply_to;

        messageDiv.innerHTML = `
            <div class="flex space-x-2 ${isReply ? 'reply-indicator pl-2' : ''}">
                <div class="flex-shrink-0">
                    ${profilePic ?
                        `<img src="${profilePic}" alt="${displayName}" class="w-8 h-8 rounded-full profile-pic object-cover">` :
                        `<div class="w-8 h-8 rounded-full bg-gray-600 profile-pic flex items-center justify-center">
                            <span class="text-xs">👤</span>
                         </div>`
                    }
                </div>
                <div class="flex-1 min-w-0">
                    <div class="flex items-baseline space-x-2">
                        <span class="username text-sm">${displayName}</span>
                        <span class="timestamp">${timeStr}</span>
                    </div>
                    ${isReply ? '<div class="text-xs text-blue-400 mb-1">↳ Reply</div>' : ''}
                    <div class="message-content text-sm">${this.formatMessageContent(message.content)}</div>
                </div>
            </div>
        `;

        return messageDiv;
    }

    getUserDisplayName(message) {
        if (message.profile) {
            return message.profile.display_name ||
                   message.profile.name ||
                   `${message.pubkey.slice(0, 8)}...`;
        }
        return `${message.pubkey.slice(0, 8)}...`;
    }

    formatMessageContent(content) {
        // Basic HTML escaping and simple formatting
        const escaped = content
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');

        // Simple URL detection (basic)
        return escaped.replace(
            /(https?:\/\/[^\s]+)/g,
            '<a href="$1" target="_blank" rel="noopener" class="text-blue-400 underline">$1</a>'
        );
    }

    async sendMessage() {
        const messageInput = document.getElementById('message-input');
        if (!messageInput || !this.isAuthenticated) return;

        const content = messageInput.value.trim();
        if (!content || content.length > 280) return;

        try {
            this.showSending();

            const response = await fetch('/api/chat/send', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    content: content,
                    reply_to: this.currentReplyTo
                })
            });

            const result = await response.json();

            if (result.success) {
                messageInput.value = '';
                this.currentReplyTo = null;
                this.updateCharCount();
                this.clearError();

                // Message will appear via WebSocket real-time update

                console.log('📤 Message sent successfully');
            } else {
                throw new Error(result.error || 'Failed to send message');
            }

        } catch (error) {
            console.error('❌ Failed to send message:', error);
            this.showError('Failed to send message. Please try again.');
        } finally {
            this.hideSending();
        }
    }

    showSending() {
        const sendButton = document.getElementById('send-button');
        if (sendButton) {
            sendButton.textContent = 'SENDING...';
            sendButton.disabled = true;
        }
    }

    hideSending() {
        const sendButton = document.getElementById('send-button');
        if (sendButton) {
            sendButton.textContent = 'SEND';
            this.updateSendButton();
        }
    }

    showLoading(message) {
        const messagesContainer = document.getElementById('chat-messages');
        if (messagesContainer && messagesContainer.children.length === 0) {
            // Only show loading if there are no existing messages or content
            messagesContainer.innerHTML = `
                <div class="text-center text-gray-500 text-sm" id="loading-indicator">
                    <div class="loading-message">${message}</div>
                </div>
            `;
        }
        // If there are existing messages, don't show loading indicator
    }

    clearLoadingMessages() {
        const messagesContainer = document.getElementById('chat-messages');
        if (messagesContainer) {
            // Remove loading indicators
            const loadingIndicator = messagesContainer.querySelector('#loading-indicator');
            if (loadingIndicator) {
                loadingIndicator.remove();
            }

            // Remove any loading messages
            const loadingMessages = messagesContainer.querySelectorAll('.loading-message');
            loadingMessages.forEach(msg => {
                const parent = msg.closest('.text-center');
                if (parent) {
                    parent.remove();
                }
            });
        }
    }

    showError(message) {
        const errorEl = document.getElementById('error-message');
        if (errorEl) {
            errorEl.textContent = message;
            errorEl.classList.remove('hidden');
        }
    }

    clearError() {
        const errorEl = document.getElementById('error-message');
        if (errorEl) {
            errorEl.classList.add('hidden');
        }
    }

    scrollToBottom() {
        const messagesContainer = document.getElementById('chat-messages');
        if (messagesContainer) {
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }
    }


    connectWebSocket() {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            return;
        }

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/api/chat/ws`;

        console.log('🔌 Connecting to WebSocket:', wsUrl);

        try {
            this.ws = new WebSocket(wsUrl);

            this.ws.onopen = () => {
                console.log('✅ WebSocket connected');
                this.wsReconnectAttempts = 0;
                this.clearError();
            };

            this.ws.onmessage = (event) => {
                try {
                    const message = JSON.parse(event.data);
                    this.handleNewMessage(message);
                } catch (error) {
                    console.error('❌ Error parsing WebSocket message:', error);
                }
            };

            this.ws.onclose = (event) => {
                console.log('🔌 WebSocket disconnected:', event.code, event.reason);
                this.ws = null;
                this.scheduleReconnect();
            };

            this.ws.onerror = (error) => {
                console.error('❌ WebSocket error:', error);
            };

        } catch (error) {
            console.error('❌ Failed to create WebSocket:', error);
            this.scheduleReconnect();
        }
    }

    scheduleReconnect() {
        if (this.wsReconnectAttempts >= this.maxReconnectAttempts) {
            console.log('❌ Max WebSocket reconnection attempts reached');
            this.showError('Connection lost. Please refresh the page.');
            return;
        }

        const delay = Math.min(1000 * Math.pow(2, this.wsReconnectAttempts), 30000);
        this.wsReconnectAttempts++;

        console.log(`🔄 Scheduling WebSocket reconnect attempt ${this.wsReconnectAttempts} in ${delay}ms`);

        setTimeout(() => {
            this.connectWebSocket();
        }, delay);
    }

    handleNewMessage(message) {
        // Check if we already have this message
        const existingMessage = this.messages.find(m => m.id === message.id);
        if (existingMessage) {
            return; // Skip duplicates
        }

        // Add to messages array
        this.messages.push(message);

        // Add to DOM
        const messagesContainer = document.getElementById('chat-messages');
        if (messagesContainer) {
            // Remove "no messages" indicator if it exists
            const noMessagesIndicator = messagesContainer.querySelector('#no-messages');
            if (noMessagesIndicator) {
                noMessagesIndicator.remove();
            }

            const messageEl = this.createMessageElement(message);
            messageEl.setAttribute('data-message-id', message.id);
            messagesContainer.appendChild(messageEl);

            // Update message count
            const messageCountEl = document.getElementById('message-count');
            if (messageCountEl) {
                const count = this.messages.length;
                messageCountEl.textContent = `${count} message${count !== 1 ? 's' : ''}`;
            }

            // Auto-scroll to bottom
            this.smartScrollToBottom();

            console.log('💬 Real-time message added:', message.id.slice(0, 8));
        }
    }

    destroy() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        console.log('🔥 Chat widget destroyed');
    }
}

// Global function to open auth in parent window (for widgets in OBS)
function openParentAuth() {
    if (window.parent && window.parent !== window) {
        // In iframe - try to communicate with parent
        window.parent.postMessage({ type: 'open_auth' }, '*');
    } else {
        // Standalone - open auth modal or redirect
        window.location.href = '/?open_auth=true';
    }
}

// Initialize chat widget when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
    window.chatWidget = new LiveChatWidget();
    console.log('🔥 Live Chat Widget initialized');
});

// Cleanup on page unload
window.addEventListener('beforeunload', function() {
    if (window.chatWidget) {
        window.chatWidget.destroy();
    }
});