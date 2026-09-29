import { useState } from 'react';

import { type CalculationProgress } from 'shared/api/instance/httpClient';

type RequestProgress = {
    startedAt: number;
    lastEventAt: number;
    progress?: CalculationProgress;
};

export const useRequestProgress = () => {
    const [state, setState] = useState<RequestProgress | null>(null);

    return {
        state,
        start: () => {
            const now = Date.now();
            setState({ startedAt: now, lastEventAt: now });
        },
        finish: () => setState(null),
        callbacks: {
            onProgress: (progress: CalculationProgress) =>
                setState((current) =>
                    current
                        ? { ...current, lastEventAt: Date.now(), progress }
                        : current
                ),
            onHeartbeat: () =>
                setState((current) =>
                    current ? { ...current, lastEventAt: Date.now() } : current
                ),
        },
    };
};
