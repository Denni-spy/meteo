// Dark mode toggle logic with localStorage persistence
// Defaults to system preference via prefers-color-scheme
// Shared across all pages
(function () {
    const STORAGE_KEY = 'meteo-dark-mode';

    function prefersDark() {
        return window.matchMedia('(prefers-color-scheme: dark)').matches;
    }

    function isDark() {
        const saved = localStorage.getItem(STORAGE_KEY);
        if (saved !== null) return saved === 'true';
        return prefersDark();
    }

    function applyTheme(dark) {
        if (dark) {
            document.documentElement.setAttribute('data-theme', 'dark');
        } else {
            document.documentElement.removeAttribute('data-theme');
        }
    }

    // Apply immediately to prevent flash of wrong theme
    applyTheme(isDark());

    document.addEventListener('DOMContentLoaded', function () {
        const toggle = document.getElementById('dark-mode-toggle');
        if (!toggle) return;

        toggle.checked = isDark();

        toggle.addEventListener('change', function () {
            applyTheme(toggle.checked);
            localStorage.setItem(STORAGE_KEY, toggle.checked);
        });

        // Listen for OS theme changes — only follow if user hasn't manually chosen
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function (e) {
            if (localStorage.getItem(STORAGE_KEY) !== null) return;
            applyTheme(e.matches);
            toggle.checked = e.matches;
        });
    });
})();
