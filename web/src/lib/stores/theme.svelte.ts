export type Theme = 'light' | 'dark' | 'system';

function getIsDark(theme: Theme): boolean {
    if (typeof window === 'undefined') return false;
    return theme === 'dark' ||
        (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
}

function applyTheme(theme: Theme) {
    if (typeof document === 'undefined') return;
    const isDark = getIsDark(theme);
    document.documentElement.classList.toggle('dark', isDark);
}

class ThemeStore {
    currentTheme = $state<Theme>('system');

    constructor() {
        if (typeof localStorage !== 'undefined') {
            const stored = localStorage.getItem('theme') as Theme | null;
            if (stored) {
                this.currentTheme = stored;
            }
        }

        applyTheme(this.currentTheme);

        $effect.root(() => {
            $effect(() => {
                if (typeof localStorage !== 'undefined') {
                    localStorage.setItem('theme', this.currentTheme);
                }
                applyTheme(this.currentTheme);
            });
        });
    }

    get theme(): Theme {
        return this.currentTheme;
    }

    get isDark(): boolean {
        return getIsDark(this.currentTheme);
    }

    setTheme(value: Theme) {
        this.currentTheme = value;
    }

    toggle() {
        const currentlyDark = getIsDark(this.currentTheme);
        this.currentTheme = currentlyDark ? 'light' : 'dark';
    }
}

export const themeStore = new ThemeStore();
