import { create } from 'zustand';

import { type DispatcherWorkspaceStore } from './types';

export const useDispatcherWorkspaceStore = create<DispatcherWorkspaceStore>(
    (set) => ({
        selectedOrderId: null,
        selectedEngineerId: null,
        filter: 'all',
        panelOpen: true,
        scheduleOpen: false,
        panelTab: 'both',
        selectOrder: (selectedOrderId) =>
            set((state) => ({
                selectedOrderId,
                panelOpen: true,
                panelTab: state.panelTab === 'both' ? 'both' : 'orders',
            })),
        selectEngineer: (selectedEngineerId) =>
            set((state) => ({
                selectedEngineerId,
                selectedOrderId:
                    state.panelTab === 'both' ? state.selectedOrderId : null,
                filter: state.panelTab === 'both' ? state.filter : 'all',
                panelOpen: true,
            })),
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
                panelTab: 'both',
                scheduleOpen: false,
            }),
    })
);
