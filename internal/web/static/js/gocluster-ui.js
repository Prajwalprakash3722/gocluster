// GoCluster Enhanced UI JavaScript

class GoClusterUI {
    constructor() {
        this.init();
    }

    init() {
        this.setupCommandPalette();
        this.setupKeyboardShortcuts();
        this.setupRealTimeUpdates();
        this.setupTooltips();
    }

    // Command Palette functionality
    setupCommandPalette() {
        const commands = [
            { name: 'Go to Dashboard', action: () => window.location.href = '/' },
            { name: 'View Nodes', action: () => window.location.href = '/nodes' },
            { name: 'View Operators', action: () => window.location.href = '/operators' },
            { name: 'View Operations History', action: () => window.location.href = '/operations' },
            { name: 'View Logs', action: () => window.location.href = '/logs' },
            { name: 'Settings', action: () => window.location.href = '/settings' },
            { name: 'Refresh Page', action: () => window.location.reload() },
            { name: 'Execute Hello Operator', action: () => this.executeOperation('hello', 'greet') },
            { name: 'Check Cluster Status', action: () => window.open('/api/status', '_blank') },
        ];

        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
                e.preventDefault();
                this.showCommandPalette(commands);
            }
        });
    }

    showCommandPalette(commands) {
        let palette = document.getElementById('commandPalette');
        if (!palette) {
            palette = this.createCommandPalette(commands);
            document.body.appendChild(palette);
        }
        
        palette.style.display = 'flex';
        const input = palette.querySelector('.palette-input');
        input.focus();
        input.value = '';
        this.updatePaletteResults(commands, '');

        // Handle input
        input.addEventListener('input', (e) => {
            this.updatePaletteResults(commands, e.target.value);
        });

        // Handle escape
        const handleEscape = (e) => {
            if (e.key === 'Escape') {
                palette.style.display = 'none';
                document.removeEventListener('keydown', handleEscape);
            }
        };
        document.addEventListener('keydown', handleEscape);

        // Handle click outside
        palette.addEventListener('click', (e) => {
            if (e.target === palette) {
                palette.style.display = 'none';
            }
        });
    }

    createCommandPalette(commands) {
        const palette = document.createElement('div');
        palette.id = 'commandPalette';
        palette.className = 'command-palette';
        palette.innerHTML = `
            <div class="palette-content">
                <input type="text" class="palette-input" placeholder="Type a command or search...">
                <div class="palette-results" id="paletteResults"></div>
            </div>
        `;
        return palette;
    }

    updatePaletteResults(commands, query) {
        const results = document.getElementById('paletteResults');
        const filtered = commands.filter(cmd => 
            cmd.name.toLowerCase().includes(query.toLowerCase())
        );

        results.innerHTML = filtered.map(cmd => `
            <div class="palette-item" onclick="goClusterUI.executePaletteCommand('${cmd.name}')">
                <i class="fas fa-terminal"></i>
                <span>${cmd.name}</span>
            </div>
        `).join('');

        // Store commands for execution
        this.currentCommands = commands;
    }

    executePaletteCommand(commandName) {
        const command = this.currentCommands.find(cmd => cmd.name === commandName);
        if (command) {
            command.action();
            document.getElementById('commandPalette').style.display = 'none';
        }
    }

    // Keyboard shortcuts
    setupKeyboardShortcuts() {
        document.addEventListener('keydown', (e) => {
            if (e.ctrlKey || e.metaKey) {
                switch(e.key) {
                    case 'r':
                        e.preventDefault();
                        window.location.reload();
                        break;
                    case '1':
                        e.preventDefault();
                        window.location.href = '/';
                        break;
                    case '2':
                        e.preventDefault();
                        window.location.href = '/nodes';
                        break;
                    case '3':
                        e.preventDefault();
                        window.location.href = '/operators';
                        break;
                    case '4':
                        e.preventDefault();
                        window.location.href = '/operations';
                        break;
                    case '5':
                        e.preventDefault();
                        window.location.href = '/logs';
                        break;
                }
            }
        });
    }

    // Real-time updates
    setupRealTimeUpdates() {
        this.startAutoRefresh();
        this.setupWebSocket();
    }

    startAutoRefresh() {
        // Refresh every 30 seconds
        setInterval(() => {
            this.updateDashboardData();
        }, 30000);
    }

    setupWebSocket() {
        // Note: This would require WebSocket support on the backend
        // For now, we'll use polling
        this.pollForUpdates();
    }

    pollForUpdates() {
        setInterval(async () => {
            try {
                const response = await fetch('/api/status');
                const data = await response.json();
                
                if (data.success) {
                    this.updateLiveElements(data.data);
                }
            } catch (error) {
                console.warn('Failed to fetch updates:', error);
            }
        }, 10000); // Poll every 10 seconds
    }

    updateLiveElements(data) {
        // Update any elements with live data
        const elements = document.querySelectorAll('[data-live]');
        elements.forEach(element => {
            const property = element.getAttribute('data-live');
            if (data[property] !== undefined) {
                element.textContent = data[property];
            }
        });
    }

    updateDashboardData() {
        const currentPath = window.location.pathname;
        
        // Only refresh if we're on a data-heavy page
        if (['/nodes', '/operators', '/operations', '/logs'].includes(currentPath)) {
            // Soft refresh - just reload the dynamic content
            this.refreshPageContent();
        }
    }

    refreshPageContent() {
        // Add loading indicator
        this.showLoading();
        
        // Reload page content after a short delay
        setTimeout(() => {
            window.location.reload();
        }, 1000);
    }

    showLoading() {
        const content = document.querySelector('.file-content');
        if (content) {
            const loading = document.createElement('div');
            loading.className = 'loading';
            loading.innerHTML = '<div class="spinner"></div>Refreshing...';
            content.appendChild(loading);
        }
    }

    // Execute operations
    async executeOperation(operator, operation, params = {}) {
        try {
            const response = await fetch('/api/operator/execute', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    operator: operator,
                    operation: operation,
                    params: params
                })
            });

            const result = await response.json();
            
            if (result.success) {
                this.showNotification('Operation executed successfully!', 'success');
                setTimeout(() => window.location.reload(), 1000);
            } else {
                this.showNotification('Operation failed: ' + result.error, 'error');
            }
        } catch (error) {
            this.showNotification('Network error: ' + error.message, 'error');
        }
    }

    // Notifications
    showNotification(message, type = 'info') {
        const notification = document.createElement('div');
        notification.className = `notification notification-${type}`;
        notification.innerHTML = `
            <div style="padding: 12px 16px; border-radius: 6px; margin: 16px; background-color: var(--bg-secondary); border: 1px solid var(--border-primary); color: var(--text-primary);">
                <i class="fas fa-${this.getNotificationIcon(type)}"></i>
                ${message}
            </div>
        `;
        
        document.body.appendChild(notification);
        
        // Auto remove after 3 seconds
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 3000);
    }

    getNotificationIcon(type) {
        switch(type) {
            case 'success': return 'check-circle';
            case 'error': return 'exclamation-circle';
            case 'warning': return 'exclamation-triangle';
            default: return 'info-circle';
        }
    }

    // Tooltips
    setupTooltips() {
        // Add tooltips to elements with data-tooltip attribute
        document.querySelectorAll('[data-tooltip]').forEach(element => {
            element.addEventListener('mouseenter', (e) => {
                this.showTooltip(e.target, e.target.getAttribute('data-tooltip'));
            });
            
            element.addEventListener('mouseleave', () => {
                this.hideTooltip();
            });
        });
    }

    showTooltip(element, text) {
        const tooltip = document.createElement('div');
        tooltip.className = 'tooltip-popup';
        tooltip.textContent = text;
        tooltip.style.cssText = `
            position: absolute;
            background-color: var(--bg-tertiary);
            color: var(--text-primary);
            padding: 8px 12px;
            border-radius: 6px;
            font-size: 12px;
            z-index: 1000;
            pointer-events: none;
            border: 1px solid var(--border-primary);
        `;
        
        document.body.appendChild(tooltip);
        
        const rect = element.getBoundingClientRect();
        tooltip.style.top = (rect.top - tooltip.offsetHeight - 8) + 'px';
        tooltip.style.left = (rect.left + rect.width / 2 - tooltip.offsetWidth / 2) + 'px';
        
        this.currentTooltip = tooltip;
    }

    hideTooltip() {
        if (this.currentTooltip) {
            this.currentTooltip.remove();
            this.currentTooltip = null;
        }
    }
}

// Initialize the UI when the page loads
document.addEventListener('DOMContentLoaded', () => {
    window.goClusterUI = new GoClusterUI();
});

// Utility functions
function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatDuration(milliseconds) {
    if (milliseconds < 1000) return milliseconds + 'ms';
    if (milliseconds < 60000) return (milliseconds / 1000).toFixed(1) + 's';
    return Math.floor(milliseconds / 60000) + 'm ' + Math.floor((milliseconds % 60000) / 1000) + 's';
}

function timeAgo(date) {
    const now = new Date();
    const diff = now - new Date(date);
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    
    if (days > 0) return days + ' day' + (days > 1 ? 's' : '') + ' ago';
    if (hours > 0) return hours + ' hour' + (hours > 1 ? 's' : '') + ' ago';
    if (minutes > 0) return minutes + ' minute' + (minutes > 1 ? 's' : '') + ' ago';
    return 'Just now';
}
