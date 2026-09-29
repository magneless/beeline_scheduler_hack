import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { ArrowLeft, MapPin } from 'lucide-react';

import { displayEngineer, formatKm } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { CalculationProgressBar } from 'shared/ui/calculationProgress';
import { MapView } from 'shared/ui/map';

import { AddressPreflight } from './AddressPreflight';
import { MapScheduleSplit } from './MapScheduleSplit';
import { PendingChangesBar } from './PendingChangesBar';
import { PlanComparison } from './PlanComparison';
import { PlanProposalDialog } from './PlanProposalDialog';
import { RouteItinerary } from './RouteItinerary';
import { WorkspaceAlerts } from './WorkspaceAlerts';
import { WorkspaceScheduleDock } from './WorkspaceScheduleDock';
import { WorkspaceSidePanel } from './WorkspaceSidePanel';
import { WorkspaceTopbar } from './WorkspaceTopbar';
import { WorkTypeBadge } from './WorkTypeBadge';
import { routeColor } from '../lib/routeColors';
import { useDispatcherWorkspace } from '../model/useDispatcherWorkspace';

export const DispatcherWorkspacePage = () => {
    const { scenarioId = '' } = useParams();
    const workspace = useDispatcherWorkspace(scenarioId);
    const [comparisonOpen, setComparisonOpen] = useState(false);
    const focused = workspace.selectedEngineerId;
    const unassignedOnly =
        workspace.panelTab !== 'crews' && workspace.filter === 'unassigned';
    return (
        <section
            className={[
                'flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden',
                'rounded-[16px] border border-border bg-white',
            ].join(' ')}
        >
            <WorkspaceTopbar
                solveMode={workspace.solveMode}
                savedSolveMode={workspace.savedSolveMode}
                onSolveMode={workspace.setSolveMode}
                snapshot={workspace.snapshot}
                metrics={workspace.plan?.metrics}
                canEvent={workspace.canEvent}
                canRebuild={
                    Boolean(workspace.plan) && workspace.canEditEngineers
                }
                canCompare={Boolean(workspace.plan)}
                buildPending={workspace.buildPending}
                eventPending={workspace.eventPending}
                progressVisible={Boolean(
                    workspace.proposalCalculation || workspace.runCalculation
                )}
                runStatus={workspace.runStatus}
                scheduleOpen={workspace.scheduleOpen}
                panelOpen={workspace.panelOpen}
                timezone={workspace.timezone}
                defaultOccurredAt={workspace.scenarioTime}
                onScenarioTime={workspace.setScenarioTime}
                hasProposal={Boolean(workspace.proposal)}
                onOpenProposal={() => workspace.setProposalOpen(true)}
                onNewOrder={workspace.handleOrderEvent!}
                onRebuild={workspace.handleBuildPlan}
                onCompare={() => setComparisonOpen(true)}
                onToggleSchedule={workspace.toggleSchedule}
                onTogglePanel={workspace.togglePanel}
            />
            {workspace.pendingChanges ? (
                <PendingChangesBar
                    queue={workspace.pendingChanges}
                    timezone={workspace.timezone}
                    addresses={workspace.addressByOrder}
                    pending={workspace.eventPending}
                    onCalculate={workspace.handleBuildPlan}
                    onUndo={workspace.undoPendingChange}
                />
            ) : null}
            {workspace.proposalCalculation || workspace.runCalculation ? (
                <CalculationProgressBar
                    state={
                        workspace.proposalCalculation ??
                        workspace.runCalculation!
                    }
                    title={
                        workspace.proposalCalculation
                            ? 'Расчёт вариантов плана'
                            : (workspace.runCalculation?.title ??
                              'Пересчёт маршрута')
                    }
                    className="mx-3 my-2 shrink-0"
                />
            ) : null}
            <div className="flex min-h-0 flex-1 max-lg:flex-col max-lg:overflow-y-auto">
                {workspace.panelOpen ? (
                    <WorkspaceSidePanel
                        hasPlan={Boolean(workspace.plan)}
                        asOf={workspace.plan?.as_of}
                        onShowUnassigned={workspace.showUnassigned}
                        panelTab={workspace.panelTab}
                        ordersCount={workspace.ordersCount}
                        crewsCount={workspace.crewsCount}
                        orders={workspace.displayOrders}
                        engineers={workspace.snapshot?.engineers ?? []}
                        scenarioId={scenarioId}
                        revision={workspace.snapshot?.revision ?? 0}
                        selectedOrderId={workspace.selectedOrderId}
                        selectedEngineerId={workspace.selectedEngineerId}
                        engineerByOrder={workspace.engineerByOrder}
                        unassigned={workspace.unassigned}
                        lateness={workspace.plan?.lateness}
                        inTransitOrderIds={workspace.inTransitOrderIds}
                        visits={workspace.visitByOrder}
                        timezone={workspace.timezone}
                        date={workspace.snapshot?.date ?? ''}
                        addressByOrder={workspace.addressByOrder}
                        remaining={workspace.plan?.equipment_remaining}
                        issues={workspace.issues}
                        defaultOccurredAt={workspace.scenarioTime}
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
                    >
                        <WorkspaceAlerts
                            issues={workspace.issues}
                            changes={workspace.plan?.changes ?? []}
                            orders={workspace.displayOrders}
                            runStatus={workspace.runStatus}
                            showRunBanner={workspace.showRunBanner}
                            onSelectOrder={workspace.selectOrder}
                            buildErrorMessage={
                                workspace.plan
                                    ? workspace.buildErrorMessage
                                    : undefined
                            }
                            onRetryBuild={workspace.handleBuildPlan}
                        />
                    </WorkspaceSidePanel>
                ) : null}
                {!workspace.plan ? (
                    workspace.snapshot && !workspace.isLoading ? (
                        <AddressPreflight
                            snapshot={workspace.snapshot}
                            pending={workspace.buildPending}
                            error={workspace.buildErrorMessage}
                            onBuild={workspace.handleBuildPlan}
                        />
                    ) : (
                        <div
                            role="status"
                            className="flex flex-1 items-center justify-center text-sm text-muted-foreground"
                        >
                            Загружаем заявки…
                        </div>
                    )
                ) : (
                    <div className="flex min-h-0 min-w-0 flex-1 flex-col max-lg:min-h-[440px] max-lg:shrink-0">
                        <div
                            className={[
                                'flex shrink-0 items-center justify-between gap-3 border-b',
                                'border-border bg-white px-5 py-3',
                            ].join(' ')}
                        >
                            <div className="min-w-0">
                                <h2 className="flex items-center gap-2 text-sm font-semibold text-foreground">
                                    {focused ? (
                                        <span
                                            className="size-2.5 shrink-0 rounded-full"
                                            style={{
                                                background: routeColor(focused),
                                            }}
                                        />
                                    ) : (
                                        <MapPin className="size-4 text-muted-foreground" />
                                    )}
                                    {focused
                                        ? `Маршрут · ${displayEngineer(focused)}`
                                        : unassignedOnly
                                          ? 'Заявки без бригады'
                                          : 'Обзор района'}
                                </h2>
                                {workspace.plan ? (
                                    <p className="mt-0.5 pl-6 text-xs text-muted-foreground">
                                        Все маршруты ·{' '}
                                        {formatKm(
                                            workspace.plan.metrics
                                                .total_distance_m
                                        )}
                                    </p>
                                ) : null}
                            </div>
                            {focused || unassignedOnly ? (
                                <Button
                                    size="sm"
                                    variant="ghost"
                                    className="shrink-0 rounded-[8px] text-muted-foreground"
                                    onClick={workspace.handleClearCrew}
                                >
                                    <ArrowLeft className="size-3.5" />
                                    Обзор района
                                </Button>
                            ) : (
                                <div
                                    className={[
                                        'hidden shrink-0 items-center gap-3 text-[11px]',
                                        'text-muted-foreground lg:flex',
                                    ].join(' ')}
                                >
                                    <span className="flex items-center gap-1.5">
                                        <i className="size-2 rounded-full bg-slate-400" />
                                        В плане
                                    </span>
                                    <span className="flex items-center gap-1.5">
                                        <i className="size-2 rounded-full bg-amber-500" />
                                        Без бригады
                                    </span>
                                </div>
                            )}
                        </div>
                        {focused ? (
                            <div
                                aria-label="Типы работ на карте"
                                className="flex shrink-0 flex-wrap gap-2 border-b border-border bg-white px-5 py-2"
                            >
                                <WorkTypeBadge type="connection" />
                                <WorkTypeBadge type="repair" />
                                <WorkTypeBadge type="emergency" />
                                <WorkTypeBadge type="additional" />
                            </div>
                        ) : null}
                        <MapScheduleSplit
                            defaultRatio={0.4}
                            map={
                                <div
                                    className="h-full"
                                    data-map-mode={
                                        focused ? 'route' : 'overview'
                                    }
                                >
                                    <MapView
                                        markers={workspace.mapModel.markers}
                                        polylines={workspace.mapModel.polylines}
                                        selectedId={workspace.selectedOrderId}
                                        fitToken={workspace.mapFitToken}
                                        onMarkerClick={workspace.onMarkerClick}
                                    />
                                </div>
                            }
                            schedule={
                                workspace.scheduleOpen && workspace.plan ? (
                                    <WorkspaceScheduleDock
                                        fillHeight
                                        lanes={workspace.lanes}
                                        focusAt={workspace.focusAt}
                                        selectedOrderId={
                                            workspace.selectedOrderId
                                        }
                                        selectedEngineerId={
                                            workspace.selectedEngineerId
                                        }
                                        onSelectOrder={workspace.selectOrder}
                                        onSelectEngineer={
                                            workspace.toggleEngineer
                                        }
                                    />
                                ) : undefined
                            }
                        />
                        {!workspace.scheduleOpen &&
                        focused &&
                        workspace.snapshot &&
                        workspace.plan ? (
                            <RouteItinerary
                                snapshot={workspace.snapshot}
                                plan={workspace.plan}
                                engineerId={focused}
                                selectedOrderId={workspace.selectedOrderId}
                                timezone={workspace.timezone}
                                onSelect={workspace.selectOrder}
                            />
                        ) : null}
                    </div>
                )}
            </div>
            {comparisonOpen && workspace.plan ? (
                <PlanComparison
                    scenarioId={scenarioId}
                    currentPlanId={workspace.plan.id}
                    initialMode={workspace.solveMode}
                    onClose={() => setComparisonOpen(false)}
                />
            ) : null}
            {workspace.proposal && workspace.proposalOpen ? (
                <PlanProposalDialog
                    unlocatedOrders={workspace.snapshot?.unlocated_orders}
                    proposal={workspace.proposal}
                    accepting={workspace.proposalAccepting}
                    onAccept={workspace.acceptProposal}
                    onClose={() => workspace.setProposalOpen(false)}
                />
            ) : null}
        </section>
    );
};

export default DispatcherWorkspacePage;
