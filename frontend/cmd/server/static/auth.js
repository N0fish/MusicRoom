function initAuthService(apiUrl) {
  window.authService = {
    apiUrl,

    getAccessToken: () => {
      return localStorage.getItem('accessToken');
    },

    getRefreshToken: () => {
      return localStorage.getItem('refreshToken');
    },

    setTokens: (accessToken, refreshToken) => {
      localStorage.setItem('accessToken', accessToken);
      if (refreshToken) {
        localStorage.setItem('refreshToken', refreshToken);
      }
    },

    logout: () => {
      localStorage.removeItem('accessToken');
      localStorage.removeItem('refreshToken');
      window.location.href = '/auth';
    },

    isLoggedIn: () => {
      const token = authService.getAccessToken();
      if (!token) {
        return false;
      }
      return !authService.isAccessTokenExpired();
    },

    isAccessTokenExpired: () => {
      const token = authService.getAccessToken();
      if (!token) {
        return true;
      }
      try {
        const decoded = jwt_decode(token);
        const now = Date.now() / 1000;
        return decoded.exp < now;
      } catch (error) {
        console.error('Failed to decode token', error);
        return true;
      }
    },

    isValidEmail: (email) => {
      const re = /^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
      return re.test(String(email).toLowerCase());
    },

    scheduleTokenRefresh: () => {
      if (!authService.isLoggedIn()) {
        return;
      }

      const token = authService.getAccessToken();
      try {
        const decoded = jwt_decode(token);
        // Refresh 1 minute before expiration
        const refreshTimeout = (decoded.exp * 1000) - Date.now() - (60 * 1000);

        if (refreshTimeout > 0) {
          setTimeout(authService.refresh, refreshTimeout);
        } else {
          authService.refresh();
        }
      } catch (error) {
        console.error('Failed to schedule token refresh', error);
      }
    },

    login: async (email, password) => {
      try {
        const res = await fetch(`${authService.apiUrl}/auth/login`, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ email, password }),
        });

        const data = await res.json();

        if (res.ok) {
          authService.setTokens(data.accessToken, data.refreshToken);
          authService.scheduleTokenRefresh();
          window.location.href = '/me';
        } else {
          return data;
        }
      } catch (error) {
        console.error('Failed to login', error);
        return { error: 'Failed to login' };
      }
    },

    signup: async (email, password) => {
      try {
        const res = await fetch(`${authService.apiUrl}/auth/register`, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ email, password }),
        });

        return await res.json();
      } catch (error) {
        console.error('Failed to signup', error);
        return { error: 'Failed to signup' };
      }
    },

    refresh: async () => {
      const refreshToken = authService.getRefreshToken();
      if (!refreshToken) {
        authService.logout();
        return;
      }

      try {
        const res = await fetch(`${authService.apiUrl}/auth/refresh`, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ refreshToken }),
        });

        if (res.ok) {
          const tokens = await res.json();
          authService.setTokens(tokens.accessToken, tokens.refreshToken);
          authService.scheduleTokenRefresh();
          if (typeof window.updateHeader === 'function') {
            window.updateHeader();
          }
        } else {
          authService.logout();
        }
      } catch (error) {
        console.error('Failed to refresh token', error);
        authService.logout();
      }
    },

    fetchWithAuth: async (url, options = {}) => {
      let res = await fetch(url, {
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${authService.getAccessToken()}`,
        },
      });

      if (res.status === 401) {
        await authService.refresh();
        res = await fetch(url, {
          ...options,
          headers: {
            ...options.headers,
            'Authorization': `Bearer ${authService.getAccessToken()}`,
          },
        });
      }

      return res;
    },

    forgotPassword: async (email) => {
      try {
        const res = await fetch(`${authService.apiUrl}/auth/forgot-password`, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ email }),
        });
        return await res.json();
      } catch (error) {
        console.error('Forgot password failed', error);
        return { error: 'Forgot password failed' };
      }
    },

    resetPassword: async (token, newPassword) => {
      try {
        const res = await fetch(`${authService.apiUrl}/auth/reset-password`, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ token, newPassword }),
        });
        return await res.json();
      } catch (error) {
        console.error('Reset password failed', error);
        return { error: 'Reset password failed' };
      }
    },

    handleOauthCallback: () => {
      if (window.location.hash) {
        const params = new URLSearchParams(window.location.hash.substring(1)); // remove #
        const accessToken = params.get('accessToken');
        const refreshToken = params.get('refreshToken');

        if (accessToken) {
          authService.setTokens(accessToken, refreshToken);
          authService.scheduleTokenRefresh();
          window.location.href = '/me';
        }
      }
    },
  };

  window.authService.handleOauthCallback();
}