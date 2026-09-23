import { create } from 'zustand';

import { type DispatcherWorkspaceStore } from './types';

export const useDispatcherWorkspaceStore = create<DispatcherWorkspaceStore>(
    (set) => ({
        selectedOrderId: null,
        selectedEngineerId: null,
        filter: 'all',
        panelOpen: true,
        scheduleOpen: true,
        panelTab: 'orders',
        selectOrder: (selectedOrderId) =>
            set({
                selectedOrderId,
                panelOpen: true,
                panelTab: 'orders',
            }),
        selectEngineer: (selectedEngineerId) =>
            set({
                selectedEngineerId,
                panelOpen: true,
            }),
        setFilter: (filter) => set({ filter }),
        setPanelTab: (panelTab) => set({ panelTab, panelOpen: true }),
        togglePanel: () => set((state) => ({ panelOpen: !state.panelOpen })),
        toggleSchedule: () =>
            set((state) => ({ scheduleOpen: !state.scheduleOpen })),
        resetSelection: () =>
            set({
                selectedOrderId: null,
                selectedEngineerId: null,
                filter: 'all',
                panelTab: 'orders',
            }),
    })
);
