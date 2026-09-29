import { type ReactNode, useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { CalendarDays, MapPinned } from 'lucide-react';

import { cn } from 'shared/lib/utils';

type AppShellProps = {
    children: ReactNode;
};

const railButtonClass = (active: boolean) =>
    cn(
        'grid size-11 shrink-0 place-items-center rounded-[12px] transition-colors',
        'outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        active
            ? 'bg-primary text-foreground'
            : 'text-muted-foreground hover:bg-muted hover:text-foreground'
    );

const lastMapKey = 'dispatcher:last-map-path';
const isMapPath = (path: string) => /^\/s\/[^/]+\/?$/.test(path);
const readLastMap = () => {
    try {
        const path = sessionStorage.getItem(lastMapKey);
        return path && isMapPath(path) ? path : null;
    } catch {
        return null;
    }
};

export const AppShell = ({ children }: AppShellProps) => {
    const { pathname } = useLocation();
    const onMap = isMapPath(pathname);
    const [lastMapPath, setLastMapPath] = useState(readLastMap);
    const mapPath = onMap ? pathname : lastMapPath;

    useEffect(() => {
        if (onMap) {
            setLastMapPath(pathname);
            try {
                sessionStorage.setItem(lastMapKey, pathname);
            } catch {
                // Keep navigation working in memory when storage is unavailable.
            }
        }
    }, [onMap, pathname]);

    return (
        <div className="flex h-dvh gap-2 bg-muted p-2 max-sm:gap-1 max-sm:p-1">
            <aside
                className={[
                    'flex w-16 shrink-0 flex-col items-center rounded-[16px]',
                    'border border-border bg-white py-3 max-sm:w-14',
                ].join(' ')}
                style={{ boxShadow: 'var(--shadow-soft)' }}
            >
                <Link
                    to="/"
                    className={cn(
                        'grid size-11 shrink-0 place-items-center rounded-[12px]',
                        'outline-none focus-visible:ring-2 focus-visible:ring-ring'
                    )}
                    aria-label="Билайн"
                    title="Подготовка рабочего дня"
                >
                    <img
                        src="/beeline.png"
                        alt=""
                        className="size-8 object-contain"
                    />
                </Link>
                <nav
                    className="mt-4 flex flex-col items-center gap-2"
                    aria-label="Основная навигация"
                >
                    <Link
                        to="/"
                        className={railButtonClass(!onMap)}
                        aria-label="Подготовка рабочего дня"
                        aria-current={!onMap ? 'page' : undefined}
                        title="Подготовка рабочего дня"
                    >
                        <CalendarDays
                            className="size-6"
                            strokeWidth={2}
                            aria-hidden="true"
                        />
                    </Link>
                    {mapPath ? (
                        <Link
                            to={mapPath}
                            className={railButtonClass(onMap)}
                            aria-label="Карта смены"
                            aria-current={onMap ? 'page' : undefined}
                            title="Карта смены"
                        >
                            <MapPinned
                                className="size-6"
                                strokeWidth={2}
                                aria-hidden="true"
                            />
                        </Link>
                    ) : null}
                </nav>
            </aside>
            {children}
        </div>
    );
};
