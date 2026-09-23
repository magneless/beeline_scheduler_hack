export const env = {
    apiUrl: import.meta.env.VITE_API_URL ?? '/api/v1',
    apiMode: import.meta.env.VITE_API_MODE ?? 'mock',
    yandexMapsKey: import.meta.env.VITE_YANDEX_MAPS_KEY ?? '',
};
