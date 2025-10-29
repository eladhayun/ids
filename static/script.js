class ChatBot {
  constructor() {
    this.conversation = [];
    this.isLoading = false;
    this.retryCount = 0;
    this.maxRetries = 3;

    this.initializeElements();
    this.attachEventListeners();
    this.checkConnection();
  }

  initializeElements() {
    this.chatMessages = document.getElementById('chatMessages');
    this.messageInput = document.getElementById('messageInput');
    this.sendButton = document.getElementById('sendButton');
    this.clearButton = document.getElementById('clearButton');
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

    // Clear chat
    this.clearButton.addEventListener('click', () => this.clearChat());

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
      this.addMessage('assistant', response);

      // Reset retry count on success
      this.retryCount = 0;

    } catch (error) {
      this.hideTypingIndicator();
      this.handleError(error);
    }
  }

  addMessage(role, content) {
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
      // For bot messages, render as markdown and process product links
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

    return data.response;
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

  clearChat() {
    if (confirm('Are you sure you want to clear the mission? This action cannot be undone.')) {
      // Keep only the initial bot message
      const initialMessage = this.chatMessages.querySelector('.message');
      this.chatMessages.innerHTML = '';
      this.chatMessages.appendChild(initialMessage);

      // Reset conversation
      this.conversation = [];

      // Reset status
      this.checkConnection();
    }
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
      return marked.parse(content);
    } catch (error) {
      console.error('Markdown parsing error:', error);
      return `<p>${this.escapeHtml(content)}</p>`;
    }
  }

  // Process product links - convert product names to clickable links
  processProductLinks(content) {
    // This regex looks for product names that might be mentioned in the response
    // and converts them to markdown links to the store
    const storeDomain = 'https://israeldefensestore.com';

    // Look for patterns that might be product names (capitalized words, possibly with numbers)
    // This is a simple heuristic - in a real implementation, you might want to be more sophisticated
    const productPattern = /\b([A-Z][a-zA-Z0-9\s\-&]+(?:Holster|Gun|Pistol|Rifle|Gear|Kit|System|Defense|Tactical|Military|Equipment|Accessory|Accessories)?)\b/g;

    return content.replace(productPattern, (match, productName) => {
      // Skip if it's already a link or if it's too short
      if (match.includes('[') || match.includes('http') || productName.length < 3) {
        return match;
      }

      // Create a search URL for the product
      const searchQuery = encodeURIComponent(productName.trim());
      const productUrl = `${storeDomain}/?s=${searchQuery}`;

      return `[${productName}](${productUrl})`;
    });
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
