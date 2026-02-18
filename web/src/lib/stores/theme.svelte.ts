export type ThemeMode = 'light' | 'dark' | 'system';
export type BrandTheme = 'tech' | 'forest';

export interface ThemeConfig {
    mode: ThemeMode;
    brand: BrandTheme;
}

function getIsDark(mode: ThemeMode): boolean {
    if (typeof window === 'undefined') return false;
    return mode === 'dark' ||
        (mode === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
}

function applyTheme(config: ThemeConfig) {
    if (typeof document === 'undefined') return;
    
    // Apply Mode
    const isDark = getIsDark(config.mode);
    document.documentElement.classList.toggle('dark', isDark);
    
    // Apply Brand
    document.documentElement.setAttribute('data-brand', config.brand);
}

function normalizeBrand(value: string | null): BrandTheme {
    return value === 'forest' ? 'forest' : 'tech';
}

class ThemeStore {
    currentConfig = $state<ThemeConfig>({ mode: 'system', brand: 'tech' });

    constructor() {
        if (typeof localStorage !== 'undefined') {
            const storedMode = localStorage.getItem('theme_mode') as ThemeMode | null;
            const storedBrand = localStorage.getItem('theme_brand');
            
            // Backward compatibility for old 'theme' key
            const oldTheme = localStorage.getItem('theme');
            if (oldTheme && !storedMode) {
                this.currentConfig.mode = oldTheme as ThemeMode;
            } else if (storedMode) {
                this.currentConfig.mode = storedMode;
            }

            this.currentConfig.brand = normalizeBrand(storedBrand);
        }

        applyTheme(this.currentConfig);

        $effect.root(() => {
            $effect(() => {
                if (typeof localStorage !== 'undefined') {
                    localStorage.setItem('theme_mode', this.currentConfig.mode);
                    localStorage.setItem('theme_brand', this.currentConfig.brand);
                }
                applyTheme(this.currentConfig);
            });
        });
    }

    get mode(): ThemeMode {
        return this.currentConfig.mode;
    }

    get brand(): BrandTheme {
        return this.currentConfig.brand;
    }

    get isDark(): boolean {
        return getIsDark(this.currentConfig.mode);
    }

    setMode(value: ThemeMode) {
        this.currentConfig.mode = value;
    }

    setBrand(value: BrandTheme) {
        this.currentConfig.brand = value;
    }

    toggle() {
        const currentlyDark = getIsDark(this.currentConfig.mode);
        this.currentConfig.mode = currentlyDark ? 'light' : 'dark';
    }
}

export const themeStore = new ThemeStore();
