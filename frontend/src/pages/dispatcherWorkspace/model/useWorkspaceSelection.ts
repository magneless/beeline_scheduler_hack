import { type TypeOrNull } from 'shared/lib/types';

import { useDispatcherWorkspaceStore } from './store';

export const useWorkspaceSelection = () => {
    const selectedOrderId = useDispatcherWorkspaceStore(
        (state) => state.selectedOrderId
    );
    const selectedEngineerId = useDispatcherWorkspaceStore(
        (state) => state.selectedEngineerId
    );
    const filter = useDispatcherWorkspaceStore((state) => state.filter);
    const selectOrder = useDispatcherWorkspaceStore(
        (state) => state.selectOrder
    );
    const selectEngineer = useDispatcherWorkspaceStore(
        (state) => state.selectEngineer
    );
    const setFilter = useDispatcherWorkspaceStore((state) => state.setFilter);
    const panelOpen = useDispatcherWorkspaceStore((state) => state.panelOpen);
    const scheduleOpen = useDispatcherWorkspaceStore(
        (state) => state.scheduleOpen
    );
    const panelTab = useDispatcherWorkspaceStore((state) => state.panelTab);
    const setPanelTab = useDispatcherWorkspaceStore(
        (state) => state.setPanelTab
    );
    const togglePanel = useDispatcherWorkspaceStore(
        (state) => state.togglePanel
    );
    const toggleSchedule = useDispatcherWorkspaceStore(
        (state) => state.toggleSchedule
    );

    const toggleEngineer = (id: TypeOrNull<string>) => {
        // Crew selection opens its tasks even when the previous tab showed
        // unassigned work. Clearing the selection restores the complete list.
        setFilter('all');
        selectOrder(null);
        selectEngineer(selectedEngineerId === id ? null : id);
    };

    const clearCrew = () => {
        toggleEngineer(selectedEngineerId);
    };

    const onMarkerClick = (id: string) => {
        if (id !== 'office') {
            selectOrder(id);
        }
    };

    return {
        selectedOrderId,
        selectedEngineerId,
        filter,
        setFilter,
        panelOpen,
        scheduleOpen,
        panelTab,
        setPanelTab,
        togglePanel,
        toggleSchedule,
        selectOrder,
        toggleEngineer,
        focusEngineer: selectEngineer,
        clearCrew,
        onMarkerClick,
    };
};
