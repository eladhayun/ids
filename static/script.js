class ChatBot {
  constructor() {
    this.conversation = [];
    this.isLoading = false;
    this.retryCount = 0;
    this.maxRetries = 3;
    this.products = {}; // Product name -> URL slug mapping for link generation
    this.gaId = null; // Google Analytics Measurement ID
    this.sessionId = this.getOrCreateSessionId(); // Session ID for conversation tracking

    this.initializeElements();
    this.attachEventListeners();
    this.checkConnection();
    this.loadAnalyticsConfig();
    // Initialize textarea height to prevent scrollbar when empty
    this.autoResizeTextarea();
  }

  // Generate or retrieve session ID from localStorage
  getOrCreateSessionId() {
    let sessionId = localStorage.getItem('chat_session_id');
    if (!sessionId) {
      // Generate UUID v4
      sessionId = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        const r = Math.random() * 16 | 0;
        const v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
      });
      localStorage.setItem('chat_session_id', sessionId);
    }
    return sessionId;
  }

  initializeElements() {
    this.chatMessages = document.getElementById('chatMessages');
    this.messageInput = document.getElementById('messageInput');
    this.sendButton = document.getElementById('sendButton');
    this.typingIndicator = document.getElementById('typingIndicator');
    this.statusIndicator = document.getElementById('statusIndicator');
    this.statusDot = this.statusIndicator.querySelector('.status-dot');
    this.statusText = this.statusIndicator.querySelector('.status-text');
    this.charCount = document.getElementById('charCount');
    this.errorModal = document.getElementById('errorModal');
    this.errorMessage = document.getElementById('errorMessage');
    this.closeErrorModal = document.getElementById('closeErrorModal');
    this.retryButton = document.getElementById('retryButton');
    this.dismissErrorButton = document.getElementById('dismissErrorButton');
    this.supportEmailModal = document.getElementById('supportEmailModal');
    this.supportEmailInput = document.getElementById('supportEmailInput');
    this.emailError = document.getElementById('emailError');
    this.sendSupportButton = document.getElementById('sendSupportButton');
    this.cancelSupportButton = document.getElementById('cancelSupportButton');
    this.closeSupportModal = document.getElementById('closeSupportModal');
  }

  attachEventListeners() {
    // Send message on button click
    this.sendButton.addEventListener('click', () => this.sendMessage());

    // Send message on Enter key (but allow Shift+Enter for new lines)
    this.messageInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        this.sendMessage();
      }
    });

    // Auto-resize textarea
    this.messageInput.addEventListener('input', () => {
      this.autoResizeTextarea();
      this.updateCharCount();
      this.updateSendButton();
    });


    // Modal controls
    this.closeErrorModal.addEventListener('click', () => this.hideErrorModal());
    this.retryButton.addEventListener('click', () => {
      this.hideErrorModal();
      this.sendMessage();
    });
    this.dismissErrorButton.addEventListener('click', () => this.hideErrorModal());

    // Support email modal controls
    this.closeSupportModal.addEventListener('click', () => this.hideSupportModal());
    this.cancelSupportButton.addEventListener('click', () => this.hideSupportModal());
    this.sendSupportButton.addEventListener('click', () => this.sendSupportRequest());
    this.supportEmailInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        this.sendSupportRequest();
      }
    });

    // Close modal on backdrop click
    this.errorModal.addEventListener('click', (e) => {
      if (e.target === this.errorModal) {
        this.hideErrorModal();
      }
    });
  }

  async checkConnection() {
    try {
      const response = await fetch('/api/healthz');
      if (response.ok) {
        this.updateStatus('connected', 'Connected');
      } else {
        this.updateStatus('error', 'Connection Error');
      }
    } catch (error) {
      this.updateStatus('error', 'Offline');
    }
  }

  async loadAnalyticsConfig() {
    try {
      const response = await fetch('/api/config');
      if (response.ok) {
        const config = await response.json();
        if (config.google_analytics_id) {
          this.gaId = config.google_analytics_id;
          console.log('Google Analytics ID loaded:', this.gaId);
          this.initializeGoogleAnalytics();
        } else {
          console.log('Google Analytics ID not configured');
        }
      }
    } catch (error) {
      console.log('Analytics config not available:', error);
    }
  }

  initializeGoogleAnalytics() {
    if (!this.gaId) {
      console.log('Google Analytics: No ID provided');
      return;
    }

    // Detect if running in iframe and get parent URL
    const isInIframe = window.self !== window.top;
    let parentUrl = null;
    try {
      if (isInIframe) {
        parentUrl = document.referrer || (window.parent !== window.self ? window.parent.location.href : null);
      }
    } catch (e) {
      // Cross-origin iframe - can't access parent URL directly
      parentUrl = document.referrer || 'cross-origin-iframe';
    }

    console.log('Initializing Google Analytics:', {
      gaId: this.gaId,
      isInIframe: isInIframe,
      parentUrl: parentUrl,
      referrer: document.referrer,
      location: window.location.href
    });

    // Initialize dataLayer and gtag function BEFORE loading the script
    window.dataLayer = window.dataLayer || [];
    window.gtag = window.gtag || function () {
      window.dataLayer.push(arguments);
    };
    window.gtag('js', new Date());

    // Configure GA4 for iframe usage with host app domain
    // Important: For cross-domain iframes, we need specific settings
    const gaConfig = {
      // Configure for iframe usage
      send_page_view: true,
      // Allow tracking in iframes (required for cross-domain iframe tracking)
      cookie_flags: 'SameSite=None;Secure',
      // Enable automatic linker for cross-domain tracking
      linker: {
        domains: ['israeldefensestore.com', 'ids.jshipster.io']
      },
      // Allow iframe tracking
      allow_google_signals: true,
      // Set custom parameters for iframe context
      custom_map: {
        'parent_url': 'parent_url',
        'in_iframe': 'in_iframe'
      }
    };

    window.gtag('config', this.gaId, gaConfig);
    console.log('Google Analytics config set:', gaConfig);

    // Load Google Analytics 4 script
    const script1 = document.createElement('script');
    script1.async = true;
    script1.src = `https://www.googletagmanager.com/gtag/js?id=${this.gaId}`;

    // Track page view immediately (before script loads) to ensure it's captured
    const pageViewParams = {
      page_title: document.title,
      page_location: window.location.href,
      parent_url: parentUrl,
      in_iframe: isInIframe,
      referrer: document.referrer
    };

    // Queue the page view event immediately
    this.trackEvent('page_view', pageViewParams);
    console.log('Page view event queued:', pageViewParams);

    // Wait for script to load, then track again to ensure it's sent
    script1.onload = () => {
      console.log('Google Analytics script loaded successfully');

      // Wait a moment for gtag to initialize
      setTimeout(() => {
        // Track page view again after script loads to ensure it's sent
        this.trackEvent('page_view', pageViewParams);
        console.log('Page view event sent to Google Analytics after script load', pageViewParams);

        // Verify gtag is working
        if (typeof window.gtag === 'function') {
          console.log('✅ gtag function is available');
        } else {
          console.error('❌ gtag function is NOT available');
        }

        // Check dataLayer
        console.log('dataLayer contents:', window.dataLayer);
        console.log('dataLayer length:', window.dataLayer ? window.dataLayer.length : 0);

        // Check if cookies are being set (GA4 uses _ga cookie)
        const hasGACookie = document.cookie.includes('_ga') || document.cookie.includes('_ga_');
        console.log('GA cookies present:', hasGACookie);
        console.log('All cookies:', document.cookie);

        // Diagnostic: Check if we're in a third-party context
        const isThirdParty = isInIframe && window.location.origin !== (parentUrl ? new URL(parentUrl).origin : null);
        console.log('Third-party context:', isThirdParty);

        // Try to manually trigger a test event
        try {
          window.gtag('event', 'test_event', {
            test: true,
            timestamp: Date.now()
          });
          console.log('✅ Test event sent successfully');
        } catch (e) {
          console.error('❌ Failed to send test event:', e);
        }
      }, 500);
    };

    script1.onerror = (error) => {
      console.error('Failed to load Google Analytics script:', error);
      // Even if script fails, try to send via Measurement Protocol as fallback
      console.warn('GA script failed to load - events may not be tracked');
    };

    document.head.appendChild(script1);
    console.log('Google Analytics script tag added to head');
  }

  trackEvent(eventName, eventParams = {}) {
    if (!this.gaId) {
      console.log('TrackEvent: No GA ID, skipping', eventName);
      return;
    }

    console.log('TrackEvent called:', eventName, eventParams);

    // Use window.gtag if available, otherwise queue for later
    if (typeof window.gtag === 'function') {
      try {
        window.gtag('event', eventName, eventParams);
        console.log('✅ Event sent via gtag:', eventName);
      } catch (error) {
        console.error('❌ Analytics tracking error:', error);
        // Fallback: push to dataLayer
        window.dataLayer = window.dataLayer || [];
        window.dataLayer.push({
          event: eventName,
          ...eventParams
        });
        console.log('Event pushed to dataLayer as fallback');
      }
    } else {
      // Queue the event if gtag isn't ready yet
      window.dataLayer = window.dataLayer || [];
      window.dataLayer.push({
        event: eventName,
        ...eventParams
      });
      console.log('Event queued in dataLayer (gtag not ready):', eventName);
    }
  }

  updateStatus(type, text) {
    this.statusDot.className = `status-dot ${type}`;
    this.statusText.textContent = text;
  }

  autoResizeTextarea() {
    this.messageInput.style.height = 'auto';
    const scrollHeight = this.messageInput.scrollHeight;
    const maxHeight = 120; // matches max-height in CSS
    const newHeight = Math.min(scrollHeight, maxHeight);
    this.messageInput.style.height = newHeight + 'px';
    // Only show scrollbar when content exceeds max height
    if (scrollHeight > maxHeight) {
      this.messageInput.style.overflowY = 'auto';
    } else {
      this.messageInput.style.overflowY = 'hidden';
    }
  }

  updateCharCount() {
    const length = this.messageInput.value.length;
    this.charCount.textContent = `${length}/1000`;

    if (length > 800) {
      this.charCount.style.color = '#ef4444';
    } else if (length > 600) {
      this.charCount.style.color = '#f59e0b';
    } else {
      this.charCount.style.color = '#6b7280';
    }
  }

  updateSendButton() {
    const hasText = this.messageInput.value.trim().length > 0;
    this.sendButton.disabled = !hasText || this.isLoading;
  }

  async sendMessage() {
    const message = this.messageInput.value.trim();
    if (!message || this.isLoading) return;

    // Add user message to conversation
    this.addMessage('user', message);
    this.messageInput.value = '';
    this.autoResizeTextarea();
    this.updateCharCount();
    this.updateSendButton();

    // Show typing indicator
    this.showTypingIndicator();

    try {
      // Send to backend
      const response = await this.sendToBackend(message);

      // Hide typing indicator
      this.hideTypingIndicator();

      // Add bot response
      this.addMessage('assistant', response.content, response.products);

      // Track chat message sent
      this.trackEvent('chat_message', {
        message_length: message.length,
        has_products: response.products && Object.keys(response.products).length > 0,
        product_count: response.products ? Object.keys(response.products).length : 0,
      });

      // Check if support escalation is requested
      if (response.request_support) {
        this.showSupportModal();
      }

      // Reset retry count on success
      this.retryCount = 0;

    } catch (error) {
      this.hideTypingIndicator();
      this.handleError(error);
    }
  }

  addMessage(role, content, products = null) {
    // Update product metadata if provided
    if (products && typeof products === 'object') {
      Object.assign(this.products, products);
    }

    const messageDiv = document.createElement('div');
    messageDiv.className = `message ${role === 'user' ? 'user-message' : 'bot-message'}`;

    const avatar = document.createElement('div');
    avatar.className = 'message-avatar';

    if (role === 'user') {
      avatar.innerHTML = `
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M12 2L13.09 8.26L20 9L13.09 9.74L12 16L10.91 9.74L4 9L10.91 8.26L12 2Z" fill="currentColor"/>
                    <path d="M12 2C13.1 2 14 2.9 14 4C14 5.1 13.1 6 12 6C10.9 6 10 5.1 10 4C10 2.9 10.9 2 12 2Z" fill="currentColor"/>
                    <path d="M12 8C15.31 8 18 10.69 18 14C18 17.31 15.31 20 12 20C8.69 20 6 17.31 6 14C6 10.69 8.69 8 12 8Z" fill="currentColor"/>
                </svg>
            `;
    } else {
      avatar.innerHTML = `
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M12 2L13.09 8.26L20 9L13.09 9.74L12 16L10.91 9.74L4 9L10.91 8.26L12 2Z" fill="currentColor"/>
                    <path d="M12 2C13.1 2 14 2.9 14 4C14 5.1 13.1 6 12 6C10.9 6 10 5.1 10 4C10 2.9 10.9 2 12 2Z" fill="currentColor"/>
                    <path d="M12 8C15.31 8 18 10.69 18 14C18 17.31 15.31 20 12 20C8.69 20 6 17.31 6 14C6 10.69 8.69 8 12 8Z" fill="currentColor"/>
                </svg>
            `;
    }

    const messageContent = document.createElement('div');
    messageContent.className = 'message-content';

    if (role === 'user') {
      // For user messages, just escape HTML
      messageContent.innerHTML = `<p>${this.escapeHtml(content)}</p>`;
    } else {
      // For bot messages, process product links and render as markdown
      const processedContent = this.processProductLinks(content);
      const markdownContent = this.renderMarkdown(processedContent);
      messageContent.innerHTML = markdownContent;
    }

    messageDiv.appendChild(avatar);
    messageDiv.appendChild(messageContent);

    this.chatMessages.appendChild(messageDiv);
    this.scrollToBottom();

    // Add to conversation array
    this.conversation.push({ role, message: content });
  }

  showTypingIndicator() {
    // Move the typing indicator to the end of the chat messages
    this.chatMessages.appendChild(this.typingIndicator);
    this.typingIndicator.style.display = 'block';
    this.scrollToBottom();
  }

  hideTypingIndicator() {
    this.typingIndicator.style.display = 'none';
  }

  async sendToBackend(message) {
    console.log('[CHAT] Sending request with sessionId:', this.sessionId);
    const requestBody = {
      conversation: this.conversation,
      session_id: this.sessionId
    };
    console.log('[CHAT] Request body:', JSON.stringify(requestBody));
    
    const response = await fetch('/api/chat', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(requestBody)
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
    }

    const data = await response.json();
    if (data.error) {
      throw new Error(data.error);
    }

    return {
      content: data.response,
      products: data.products || {},
      request_support: data.request_support || false
    };
  }

  handleError(error) {
    console.error('Chat error:', error);

    // Update status
    this.updateStatus('error', 'Connection Error');

    // Show error message
    this.addMessage('assistant', 'Sorry, I encountered an error. Please try again.');

    // Show error modal for retry option
    if (this.retryCount < this.maxRetries) {
      this.showErrorModal(error.message);
    }

    this.retryCount++;
  }

  showErrorModal(message) {
    this.errorMessage.textContent = message;
    this.errorModal.style.display = 'flex';
  }

  hideErrorModal() {
    this.errorModal.style.display = 'none';
  }


  scrollToBottom() {
    setTimeout(() => {
      this.chatMessages.scrollTop = this.chatMessages.scrollHeight;
    }, 100);
  }

  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  // Process product links - convert product names to clickable links using URL slugs
  processProductLinks(content) {
    const storeDomain = 'https://israeldefensestore.com';
    let processedContent = content;

    // Process each product in our metadata
    // Sort by length (longest first) to match longer product names before shorter ones
    const sortedProducts = Object.entries(this.products).sort((a, b) => b[0].length - a[0].length);

    for (const [productName, urlSlug] of sortedProducts) {
      if (!urlSlug || urlSlug.trim() === '') {
        continue;
      }

      // Create proper product URL using the URL slug
      // Format: https://israeldefensestore.com/product/{url-slug}/
      const productUrl = `${storeDomain}/product/${encodeURIComponent(urlSlug)}/`;

      // Match product name in the text (case-insensitive, whole word matching)
      // Use a regex that matches the product name but not if it's already inside a link
      // Escape the product name for regex, but allow flexible matching
      const escapedName = this.escapeRegex(productName);
      const regex = new RegExp(`\\b(${escapedName})(?![^\\[]*\\])`, 'gi');

      processedContent = processedContent.replace(regex, (match, productNameMatch) => {
        // Check if we're already inside a markdown link by looking backwards and forwards
        const matchIndex = processedContent.indexOf(match);
        const beforeMatch = processedContent.substring(Math.max(0, matchIndex - 50), matchIndex);
        const afterMatch = processedContent.substring(matchIndex + match.length, matchIndex + match.length + 10);

        // Skip if already inside a markdown link
        if (beforeMatch.includes('[') || afterMatch.includes(']') || beforeMatch.includes('(') || afterMatch.includes(')')) {
          return match;
        }

        // Create markdown link: [Product Name](URL)
        return `[${productNameMatch}](${productUrl})`;
      });
    }

    return processedContent;
  }

  // Escape special regex characters
  escapeRegex(str) {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  // Render markdown content
  renderMarkdown(content) {
    if (typeof marked === 'undefined') {
      // Fallback if marked library is not loaded
      return `<p>${this.escapeHtml(content)}</p>`;
    }

    // Configure marked options
    marked.setOptions({
      breaks: true, // Convert line breaks to <br>
      gfm: true,    // GitHub Flavored Markdown
      sanitize: false, // Allow HTML (we'll sanitize manually)
    });

    try {
      const markdownContent = marked.parse(content);
      // Add target="_blank" to all links
      return markdownContent.replace(/<a\s+([^>]*?)href\s*=\s*["']([^"']*?)["']([^>]*?)>/gi, '<a $1href="$2"$3 target="_blank" rel="noopener noreferrer">');
    } catch (error) {
      console.error('Markdown parsing error:', error);
      return `<p>${this.escapeHtml(content)}</p>`;
    }
  }

  validateEmail(email) {
    const re = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return re.test(email);
  }

  showSupportModal() {
    this.supportEmailInput.value = '';
    this.emailError.style.display = 'none';
    this.supportEmailModal.style.display = 'flex';
    // Focus on email input
    setTimeout(() => this.supportEmailInput.focus(), 100);
  }

  hideSupportModal() {
    this.supportEmailModal.style.display = 'none';
    this.supportEmailInput.value = '';
    this.emailError.style.display = 'none';
  }

  async sendSupportRequest() {
    const email = this.supportEmailInput.value.trim();

    // Validate email format
    if (!email) {
      this.emailError.textContent = 'Please enter your email address';
      this.emailError.style.display = 'block';
      return;
    }

    if (!this.validateEmail(email)) {
      this.emailError.textContent = 'Please enter a valid email address';
      this.emailError.style.display = 'block';
      return;
    }

    // Hide error message
    this.emailError.style.display = 'none';

    // Disable button during request
    this.sendSupportButton.disabled = true;
    this.sendSupportButton.textContent = 'Sending...';

    try {
      const response = await fetch('/api/chat/request-support', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          conversation: this.conversation,
          customer_email: email,
          session_id: this.sessionId
        })
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
      }

      const data = await response.json();
      if (!data.success) {
        throw new Error(data.error || 'Failed to send support request');
      }

      // Show success message
      this.addMessage('assistant', data.message || 'Your conversation has been sent to our support team. We\'ll get back to you soon!');
      this.hideSupportModal();

      // Track support request
      this.trackEvent('support_request', {
        conversation_length: this.conversation.length,
      });

    } catch (error) {
      console.error('Support request error:', error);
      this.emailError.textContent = error.message || 'Failed to send support request. Please try again.';
      this.emailError.style.display = 'block';
    } finally {
      this.sendSupportButton.disabled = false;
      this.sendSupportButton.textContent = 'Send to Support';
    }
  }

}

// Initialize the chat bot when the page loads
document.addEventListener('DOMContentLoaded', () => {
  new ChatBot();

  // Close button handler for embedded mode
  const closeButton = document.getElementById('closeButton');
  if (closeButton) {
    closeButton.addEventListener('click', () => {
      // Send message to parent window to close the chat
      window.parent.postMessage({ type: 'ids-close-chat' }, '*');
    });
  }
});

// Handle page visibility changes to check connection
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) {
    // Page became visible, check connection
    const chatBot = window.chatBot;
    if (chatBot) {
      chatBot.checkConnection();
    }
  }
});
