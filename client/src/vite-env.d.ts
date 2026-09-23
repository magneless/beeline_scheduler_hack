/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_API_URL: string;
    readonly VITE_API_MODE: 'mock' | 'live';
    readonly VITE_YANDEX_MAPS_KEY: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}
