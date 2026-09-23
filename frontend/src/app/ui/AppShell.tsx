import { type ReactNode, useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { LayoutGrid, Map } from 'lucide-react';

import { cn } from 'shared/lib/utils';

type AppShellProps = {
    children: ReactNode;
};

const railButtonClass = (active: boolean) =>
    cn(
        'flex size-11 items-center justify-center rounded-[16px] transition-colors',
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
        <div className="flex h-screen gap-2 bg-muted p-2 max-sm:gap-1 max-sm:p-1">
            <aside
                className={[
                    'flex w-14 shrink-0 flex-col items-center gap-3 rounded-[16px]',
                    'border border-border bg-white py-3 max-sm:w-11',
                ].join(' ')}
                style={{ boxShadow: 'var(--shadow-soft)' }}
            >
                <Link
                    to="/"
                    className="mb-2 flex size-11 items-center justify-center"
                    aria-label="Билайн"
                >
                    <img
                        src="/beeline.png"
                        alt="Билайн"
                        className="size-9 object-contain"
                    />
                </Link>
                <Link
                    to="/"
                    className={railButtonClass(!onMap)}
                    aria-label="Районы"
                    aria-current={!onMap ? 'page' : undefined}
                >
                    <LayoutGrid className="size-5" />
                </Link>
                {mapPath ? (
                    <Link
                        to={mapPath}
                        className={railButtonClass(onMap)}
                        aria-label="Карта смены"
                        aria-current={onMap ? 'page' : undefined}
                        title="Вернуться к карте смены"
                    >
                        <Map className="size-5" />
                    </Link>
                ) : null}
            </aside>
            {children}
        </div>
    );
};
