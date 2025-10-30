class ChatBot {
  constructor() {
    this.conversation = [];
    this.isLoading = false;
    this.retryCount = 0;
    this.maxRetries = 3;
    this.products = {}; // Product name -> URL slug mapping for link generation

    this.initializeElements();
    this.attachEventListeners();
    this.checkConnection();
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

  updateStatus(type, text) {
    this.statusDot.className = `status-dot ${type}`;
    this.statusText.textContent = text;
  }

  autoResizeTextarea() {
    this.messageInput.style.height = 'auto';
    this.messageInput.style.height = Math.min(this.messageInput.scrollHeight, 120) + 'px';
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
    const response = await fetch('/api/chat', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        conversation: this.conversation
      })
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
      products: data.products || {}
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

}

// Initialize the chat bot when the page loads
document.addEventListener('DOMContentLoaded', () => {
  new ChatBot();
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
