import { useParams } from 'react-router-dom';
import { ArrowLeft, MapPin } from 'lucide-react';

import { displayEngineer } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { MapView } from 'shared/ui/map';

import { BuildPlanPrompt } from './BuildPlanPrompt';
import { RouteItinerary } from './RouteItinerary';
import { WorkspaceAlerts } from './WorkspaceAlerts';
import { WorkspaceScheduleDock } from './WorkspaceScheduleDock';
import { WorkspaceSidePanel } from './WorkspaceSidePanel';
import { WorkspaceTopbar } from './WorkspaceTopbar';
import { routeColor } from '../lib/routeColors';
import { useDispatcherWorkspace } from '../model/useDispatcherWorkspace';

export const DispatcherWorkspacePage = () => {
    const { scenarioId = '' } = useParams();
    const workspace = useDispatcherWorkspace(scenarioId);
    const focused = workspace.selectedEngineerId;
    const unassignedOnly =
        workspace.panelTab === 'orders' && workspace.filter === 'unassigned';
    return (
        <section
            className={[
                'flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden',
                'rounded-[16px] border border-slate-200 bg-white',
            ].join(' ')}
        >
            <WorkspaceTopbar
                snapshot={workspace.snapshot}
                metrics={workspace.plan?.metrics}
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
                onNewOrder={workspace.handleOrderEvent!}
                onRebuild={workspace.handleBuildPlan}
                onToggleSchedule={workspace.toggleSchedule}
                onTogglePanel={workspace.togglePanel}
            />
            <div className="flex min-h-0 flex-1 max-sm:flex-col max-sm:overflow-y-auto">
                {workspace.panelOpen ? (
                    <WorkspaceSidePanel
                        hasPlan={Boolean(workspace.plan)}
                        onShowUnassigned={workspace.showUnassigned}
                        panelTab={workspace.panelTab}
                        ordersCount={workspace.ordersCount}
                        crewsCount={workspace.crewsCount}
                        orders={workspace.snapshot?.orders ?? []}
                        engineers={workspace.snapshot?.engineers ?? []}
                        scenarioId={scenarioId}
                        revision={workspace.snapshot?.revision ?? 0}
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
                    >
                        <WorkspaceAlerts
                            issues={workspace.issues}
                            changes={workspace.plan?.changes ?? []}
                            orders={workspace.snapshot?.orders ?? []}
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
                <div className="flex min-h-0 min-w-0 flex-1 flex-col max-sm:min-h-[440px] max-sm:shrink-0">
                    <div
                        className={[
                            'flex shrink-0 items-center justify-between gap-3 border-b',
                            'border-slate-200 bg-white px-5 py-3',
                        ].join(' ')}
                    >
                        <div className="min-w-0">
                            <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-800">
                                {focused ? (
                                    <span
                                        className="size-2.5 shrink-0 rounded-full"
                                        style={{
                                            background: routeColor(focused),
                                        }}
                                    />
                                ) : (
                                    <MapPin className="size-4 text-slate-400" />
                                )}
                                {focused
                                    ? `Маршрут · ${displayEngineer(focused)}`
                                    : unassignedOnly
                                      ? 'Заявки без бригады'
                                      : 'Обзор района'}
                            </h2>
                            <p className="mt-1 text-xs text-slate-500">
                                {focused
                                    ? 'Номера на карте — порядок визитов. Нажмите на остановку для действий.'
                                    : unassignedOnly
                                      ? 'Выберите заявку, чтобы посмотреть причину и доступные действия.'
                                      : 'Выберите бригаду слева, чтобы увидеть её маршрут.'}
                            </p>
                        </div>
                        {focused || unassignedOnly ? (
                            <Button
                                size="sm"
                                variant="ghost"
                                className="shrink-0 rounded-[8px] text-slate-500"
                                onClick={workspace.handleClearCrew}
                            >
                                <ArrowLeft className="size-3.5" />
                                Обзор района
                            </Button>
                        ) : (
                            <div className="hidden shrink-0 items-center gap-3 text-[11px] text-slate-500 lg:flex">
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
                    <div
                        className="relative min-h-[240px] min-w-0 flex-1"
                        data-map-mode={focused ? 'route' : 'overview'}
                    >
                        <MapView
                            markers={workspace.mapModel.markers}
                            polylines={workspace.mapModel.polylines}
                            selectedId={workspace.selectedOrderId}
                            fitToken={workspace.mapFitToken}
                            onMarkerClick={workspace.onMarkerClick}
                        />
                        {workspace.plan ||
                        workspace.eventPending ||
                        workspace.buildPending ? null : (
                            <BuildPlanPrompt
                                pending={workspace.buildPending}
                                statusLabel={workspace.runStatusLabel}
                                errorMessage={workspace.buildErrorMessage}
                                onBuild={workspace.handleBuildPlan}
                            />
                        )}
                        {workspace.buildPending && !workspace.plan ? (
                            <div className="absolute inset-0 z-10 flex items-center justify-center bg-white/70">
                                <p
                                    role="status"
                                    className="rounded-[12px] bg-white px-5 py-4 text-sm shadow-sm"
                                >
                                    {workspace.runStatusLabel ??
                                        'Распределяем заявки и строим маршруты…'}
                                </p>
                            </div>
                        ) : null}
                    </div>
                    {workspace.scheduleOpen && workspace.plan ? (
                        <WorkspaceScheduleDock
                            lanes={workspace.lanes}
                            focusAt={workspace.focusAt}
                            selectedOrderId={workspace.selectedOrderId}
                            selectedEngineerId={workspace.selectedEngineerId}
                            onSelectOrder={workspace.selectOrder}
                            onSelectEngineer={workspace.toggleEngineer}
                        />
                    ) : focused && workspace.snapshot && workspace.plan ? (
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
            </div>
        </section>
    );
};

export default DispatcherWorkspacePage;
