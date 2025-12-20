class AdminSessions {
  constructor() {
    this.authToken = localStorage.getItem('admin_auth_token');
    this.currentPage = 0;
    this.pageSize = 20;
    this.israelTZ = 'Asia/Jerusalem';

    this.initializeElements();
    this.attachEventListeners();

    if (this.authToken) {
      this.showDashboard();
      this.loadSessions();
    } else {
      this.showLogin();
    }
  }

  initializeElements() {
    this.loginScreen = document.getElementById('loginScreen');
    this.adminDashboard = document.getElementById('adminDashboard');
    this.loginForm = document.getElementById('loginForm');
    this.loginError = document.getElementById('loginError');
    this.loginBtn = document.getElementById('loginBtn');
    this.logoutBtn = document.getElementById('logoutBtn');
    this.sessionsTableBody = document.getElementById('sessionsTableBody');
    this.pagination = document.getElementById('pagination');
    this.sessionModal = document.getElementById('sessionModal');
    this.sessionModalBody = document.getElementById('sessionModalBody');
    this.closeModal = document.getElementById('closeModal');
    this.emailModal = document.getElementById('emailModal');
    this.emailModalBody = document.getElementById('emailModalBody');
    this.closeEmailModal = document.getElementById('closeEmailModal');
  }

  attachEventListeners() {
    this.loginForm.addEventListener('submit', (e) => {
      e.preventDefault();
      this.handleLogin();
    });

    this.logoutBtn.addEventListener('click', () => {
      this.handleLogout();
    });

    this.closeModal.addEventListener('click', () => {
      this.hideSessionModal();
    });

    this.closeEmailModal.addEventListener('click', () => {
      this.hideEmailModal();
    });

    // Close modals on backdrop click
    this.sessionModal.addEventListener('click', (e) => {
      if (e.target === this.sessionModal) {
        this.hideSessionModal();
      }
    });

    this.emailModal.addEventListener('click', (e) => {
      if (e.target === this.emailModal) {
        this.hideEmailModal();
      }
    });
  }

  showLogin() {
    this.loginScreen.style.display = 'block';
    this.adminDashboard.style.display = 'none';
  }

  showDashboard() {
    this.loginScreen.style.display = 'none';
    this.adminDashboard.style.display = 'block';
  }

  async handleLogin() {
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;

    this.loginBtn.disabled = true;
    this.loginBtn.textContent = 'Logging in...';
    this.loginError.style.display = 'none';

    try {
      const response = await fetch('/api/admin/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password }),
      });

      const data = await response.json();

      if (!response.ok || !data.success) {
        throw new Error(data.error || 'Login failed');
      }

      this.authToken = data.token;
      localStorage.setItem('admin_auth_token', this.authToken);
      this.showDashboard();
      this.loadSessions();
    } catch (error) {
      this.loginError.textContent = error.message || 'Login failed. Please try again.';
      this.loginError.style.display = 'block';
    } finally {
      this.loginBtn.disabled = false;
      this.loginBtn.textContent = 'Login';
    }
  }

  handleLogout() {
    this.authToken = null;
    localStorage.removeItem('admin_auth_token');
    this.showLogin();
    document.getElementById('loginForm').reset();
  }

  async loadSessions() {
    const offset = this.currentPage * this.pageSize;

    try {
      const response = await fetch(`/api/admin/sessions?limit=${this.pageSize}&offset=${offset}`, {
        headers: {
          'Authorization': `Bearer ${this.authToken}`,
        },
      });

      if (!response.ok) {
        if (response.status === 401) {
          this.handleLogout();
          return;
        }
        throw new Error(`Failed to load sessions: ${response.statusText}`);
      }

      const data = await response.json();
      
      // Debug logging
      console.log('Sessions API response:', data);
      
      // Check if response has error
      if (data.error) {
        throw new Error(data.error);
      }
      
      // Ensure sessions array exists (handle null/undefined)
      if (!data || !data.sessions) {
        data = { sessions: [], total: 0, limit: this.pageSize, offset: 0, has_more: false };
      }
      
      this.renderSessions(data);
      this.renderPagination(data);
    } catch (error) {
      this.sessionsTableBody.innerHTML = `<tr><td colspan="4" class="error-message">Error: ${error.message}</td></tr>`;
    }
  }

  renderSessions(data) {
    if (!data || !data.sessions || data.sessions.length === 0) {
      this.sessionsTableBody.innerHTML = '<tr><td colspan="4" class="loading">No sessions found</td></tr>';
      return;
    }

    this.sessionsTableBody.innerHTML = data.sessions.map(session => {
      const createdAt = this.formatDateTime(session.created_at);
      const emailBadge = session.email_sent
        ? '<span class="email-badge sent">Sent</span>'
        : '<span class="email-badge not-sent">Not Sent</span>';

      return `
        <tr onclick="adminSessions.viewSession('${session.session_id}')">
          <td>${createdAt}</td>
          <td><span class="session-id">${session.session_id}</span></td>
          <td>${session.message_count || 0}</td>
          <td>${emailBadge}</td>
        </tr>
      `;
    }).join('');
  }

  renderPagination(data) {
    if (!data || data.total === undefined) {
      this.pagination.innerHTML = '';
      return;
    }
    
    const totalPages = Math.ceil(data.total / this.pageSize);
    const currentPage = this.currentPage;

    let html = '';

    // Previous button
    html += `<button onclick="adminSessions.previousPage()" ${currentPage === 0 ? 'disabled' : ''}>Previous</button>`;

    // Page info
    html += `<span>Page ${currentPage + 1} of ${totalPages} (${data.total} total)</span>`;

    // Next button
    html += `<button onclick="adminSessions.nextPage()" ${!data.has_more ? 'disabled' : ''}>Next</button>`;

    this.pagination.innerHTML = html;
  }

  previousPage() {
    if (this.currentPage > 0) {
      this.currentPage--;
      this.loadSessions();
    }
  }

  nextPage() {
    this.currentPage++;
    this.loadSessions();
  }

  async viewSession(sessionId) {
    try {
      const response = await fetch(`/api/admin/sessions/${sessionId}`, {
        headers: {
          'Authorization': `Bearer ${this.authToken}`,
        },
      });

      if (!response.ok) {
        throw new Error(`Failed to load session: ${response.statusText}`);
      }

      const data = await response.json();
      this.renderSessionModal(data);
    } catch (error) {
      this.sessionModalBody.innerHTML = `<div class="error-message">Error: ${error.message}</div>`;
      this.showSessionModal();
    }
  }

  renderSessionModal(sessionDetail) {
    const session = sessionDetail.session;
    const messages = sessionDetail.messages || [];
    const createdAt = this.formatDateTime(session.created_at);
    const updatedAt = this.formatDateTime(session.updated_at);

    let html = `
      <div style="margin-bottom: 1.5rem;">
        <h3 style="margin-bottom: 0.5rem; color: #1a2e1a;">Session Information</h3>
        <p><strong>Session ID:</strong> <span class="session-id">${session.session_id}</span></p>
        <p><strong>Created:</strong> ${createdAt}</p>
        <p><strong>Last Updated:</strong> ${updatedAt}</p>
        <p><strong>Messages:</strong> ${session.message_count || 0}</p>
        <p><strong>Email Sent:</strong> ${session.email_sent ? 'Yes' : 'No'}</p>
      </div>
    `;

    if (session.email_sent) {
      html += `
        <button class="btn view-email-btn" onclick="adminSessions.viewEmail('${session.session_id}')">
          View Email HTML
        </button>
      `;
    }

    html += `
      <div style="margin-top: 2rem;">
        <h3 style="margin-bottom: 1rem; color: #1a2e1a;">Conversation</h3>
        <div class="message-list">
    `;

    messages.forEach(msg => {
      const role = msg.role === 'user' ? 'user' : 'assistant';
      const roleLabel = msg.role === 'user' ? 'User' : 'AI Assistant';
      const time = this.formatDateTime(msg.created_at);

      html += `
        <div class="message-item ${role}">
          <div class="message-header">
            <span>${roleLabel}</span>
            <span class="message-time">${time}</span>
          </div>
          <div class="message-content">${this.escapeHtml(msg.message)}</div>
        </div>
      `;
    });

    html += `
        </div>
      </div>
    `;

    this.sessionModalBody.innerHTML = html;
    this.showSessionModal();
  }

  async viewEmail(sessionId) {
    try {
      const response = await fetch(`/api/admin/sessions/${sessionId}/email`, {
        headers: {
          'Authorization': `Bearer ${this.authToken}`,
        },
      });

      if (!response.ok) {
        throw new Error(`Failed to load email: ${response.statusText}`);
      }

      const html = await response.text();
      this.emailModalBody.innerHTML = html;
      this.showEmailModal();
    } catch (error) {
      this.emailModalBody.innerHTML = `<div class="error-message">Error: ${error.message}</div>`;
      this.showEmailModal();
    }
  }

  showSessionModal() {
    this.sessionModal.classList.add('active');
  }

  hideSessionModal() {
    this.sessionModal.classList.remove('active');
  }

  showEmailModal() {
    this.emailModal.classList.add('active');
  }

  hideEmailModal() {
    this.emailModal.classList.remove('active');
  }

  formatDateTime(dateString) {
    const date = new Date(dateString);
    return date.toLocaleString('en-US', {
      timeZone: this.israelTZ,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  }

  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

// Initialize the admin sessions manager
let adminSessions;
document.addEventListener('DOMContentLoaded', () => {
  adminSessions = new AdminSessions();
});

