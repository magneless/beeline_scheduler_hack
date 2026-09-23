import { type Issue } from 'shared/api';
import { issueCodeLabel } from 'shared/lib/config';
import { cn } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { workspaceCopy } from '../lib/config';

type IssuesBannerProps = {
    issues: Issue[];
    onSelect?: (entityId: string) => void;
};

const issueMessage = (issue: Issue) => {
    const address = issue.message.match(/"([^"]+)"/)?.[1];
    if (issue.message.startsWith('ambiguous address')) {
        return `Уточните адрес: ${address ?? 'найдено несколько совпадений'}`;
    }
    if (issue.message.startsWith('no geocoding results')) {
        return `Адрес не найден: ${address ?? 'уточните улицу и дом'}`;
    }
    if (issue.code === 'DEMO_OFFICE_OVERRIDE') {
        return 'В демосценарии использован подтверждённый адрес офиса.';
    }
    return issue.message;
};

export const IssuesBanner = ({ issues, onSelect }: IssuesBannerProps) => {
    if (!issues.length) {
        return null;
    }

    return (
        <div className={cn('bg-white px-2 py-2', 'overflow-y-auto')}>
            <p className="text-[11px] font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                {workspaceCopy.issuesTitle}
            </p>
            <div className="mt-2 flex flex-col gap-1">
                {issues.map((issue, index) => {
                    const title = issueCodeLabel[issue.code] ?? 'Замечание';
                    const clickable = issue.entity_id && onSelect;
                    const handleClick = () => {
                        if (issue.entity_id && onSelect) {
                            onSelect(issue.entity_id);
                        }
                    };

                    return (
                        <Button
                            key={`${issue.code}-${issue.entity_id ?? issue.source_row}-${index}`}
                            type="button"
                            variant="ghost"
                            disabled={!clickable}
                            className={cn(
                                'h-auto w-full justify-start rounded-[16px] px-2 py-1.5',
                                'text-left text-xs whitespace-normal',
                                clickable ? 'hover:bg-muted' : 'cursor-default'
                            )}
                            onClick={handleClick}
                        >
                            <strong className="font-semibold">{title}</strong>
                            {issue.source_row ? (
                                <span className="text-muted-foreground">
                                    {` · строка ${issue.source_row}`}
                                </span>
                            ) : null}
                            <span className="mt-0.5 block text-muted-foreground">
                                {issueMessage(issue)}
                            </span>
                        </Button>
                    );
                })}
            </div>
        </div>
    );
};
