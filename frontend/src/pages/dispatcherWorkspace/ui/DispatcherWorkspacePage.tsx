import { useParams } from 'react-router-dom';

import { MapView } from 'shared/ui/map';

import { BuildPlanPrompt } from './BuildPlanPrompt';
import { WorkspaceAlerts } from './WorkspaceAlerts';
import { WorkspaceScheduleDock } from './WorkspaceScheduleDock';
import { WorkspaceSidePanel } from './WorkspaceSidePanel';
import { WorkspaceTopbar } from './WorkspaceTopbar';
import { useDispatcherWorkspace } from '../model/useDispatcherWorkspace';

export const DispatcherWorkspacePage = () => {
    const { scenarioId = '' } = useParams();
    const workspace = useDispatcherWorkspace(scenarioId);

    return (
        <section
            className="relative min-h-0 min-w-0 flex-1 overflow-hidden rounded-[32px] bg-card"
            style={{ boxShadow: 'var(--shadow-soft)' }}
        >
            <div className="absolute inset-0">
                <MapView
                    markers={workspace.mapModel.markers}
                    polylines={workspace.mapModel.polylines}
                    selectedId={workspace.selectedOrderId}
                    fitToken={workspace.mapFitToken}
                    onMarkerClick={workspace.onMarkerClick}
                />
            </div>
            <WorkspaceTopbar
                snapshot={workspace.snapshot}
                metrics={workspace.plan?.metrics}
                compare={workspace.compareMetrics}
                compareSource={workspace.compareSource}
                canEvent={workspace.canEvent}
                canRebuild={
                    Boolean(workspace.plan) && workspace.canEditEngineers
                }
                buildPending={workspace.buildPending}
                eventPending={workspace.eventPending}
                runStatus={workspace.runStatus}
                scheduleOpen={workspace.scheduleOpen}
                panelOpen={workspace.panelOpen}
                timezone={workspace.timezone}
                defaultOccurredAt={workspace.occurredAtDefault}
                onEmergency={workspace.handleEmergency}
                onRebuild={workspace.handleBuildPlan}
                onToggleSchedule={workspace.toggleSchedule}
                onTogglePanel={workspace.togglePanel}
            />
            <WorkspaceAlerts
                issues={workspace.issues}
                changes={workspace.plan?.changes ?? []}
                orders={workspace.snapshot?.orders ?? []}
                runStatus={workspace.runStatus}
                showRunBanner={workspace.showRunBanner}
                onSelectOrder={workspace.selectOrder}
            />
            {workspace.plan ||
            workspace.eventPending ||
            workspace.buildPending ? null : (
                <BuildPlanPrompt
                    pending={workspace.buildPending}
                    statusLabel={workspace.runStatusLabel}
                    onBuild={workspace.handleBuildPlan}
                />
            )}
            {workspace.panelOpen ? (
                <WorkspaceSidePanel
                    panelTab={workspace.panelTab}
                    ordersCount={workspace.ordersCount}
                    crewsCount={workspace.crewsCount}
                    orders={workspace.snapshot?.orders ?? []}
                    engineers={workspace.snapshot?.engineers ?? []}
                    selectedOrderId={workspace.selectedOrderId}
                    selectedEngineerId={workspace.selectedEngineerId}
                    engineerByOrder={workspace.engineerByOrder}
                    unassigned={workspace.unassigned}
                    visits={workspace.visitByOrder}
                    timezone={workspace.timezone}
                    date={workspace.snapshot?.date ?? ''}
                    addressByOrder={workspace.addressByOrder}
                    remaining={workspace.plan?.equipment_remaining}
                    issues={workspace.issues}
                    defaultOccurredAt={workspace.occurredAtDefault}
                    filter={workspace.filter}
                    distances={workspace.distances}
                    baselineDistances={workspace.baselineDistances}
                    assignedCounts={workspace.assignedCounts}
                    canEditEngineers={workspace.canEditEngineers}
                    canEvent={Boolean(workspace.plan)}
                    pending={workspace.crewPending}
                    statusPending={workspace.eventPending}
                    onPanelTab={workspace.setPanelTab}
                    onFilter={workspace.setFilter}
                    onSelectOrder={workspace.selectOrder}
                    onSelectEngineer={workspace.toggleEngineer}
                    onClearCrew={workspace.handleClearCrew}
                    onOrderEvent={workspace.handleOrderEvent}
                    onPatchEngineer={workspace.handlePatchEngineer}
                    onUnavailable={workspace.handleEngineerUnavailable}
                />
            ) : null}
            {workspace.scheduleOpen && workspace.plan ? (
                <WorkspaceScheduleDock
                    panelOpen={workspace.panelOpen}
                    lanes={workspace.lanes}
                    focusAt={workspace.focusAt}
                    selectedOrderId={workspace.selectedOrderId}
                    selectedEngineerId={workspace.selectedEngineerId}
                    onSelectOrder={workspace.selectOrder}
                    onSelectEngineer={workspace.toggleEngineer}
                />
            ) : null}
        </section>
    );
};

export default DispatcherWorkspacePage;
