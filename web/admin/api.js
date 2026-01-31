// API utility functions for Remote File Manager

const API = {
    baseURL: '',

    async request(url, options = {}) {
        const defaultOptions = {
            credentials: 'include',
            headers: {
                'Content-Type': 'application/json',
            },
        };

        const response = await fetch(url, { ...defaultOptions, ...options });
        return response;
    },

    async login(password) {
        return this.request('/admin/login', {
            method: 'POST',
            body: JSON.stringify({ password }),
        });
    },

    async logout() {
        return this.request('/admin/logout', {
            method: 'POST',
        });
    },

    async getDevices() {
        return this.request('/api/devices');
    },
};

// Export for use in other scripts
if (typeof module !== 'undefined' && module.exports) {
    module.exports = API;
}
