import { type Issue } from 'shared/api';
import { issueCodeLabel } from 'shared/lib/config';
import { cn } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { workspaceCopy } from '../lib/config';

type IssuesBannerProps = {
    issues: Issue[];
    onSelect?: (entityId: string) => void;
};

export const IssuesBanner = ({ issues, onSelect }: IssuesBannerProps) => {
    if (!issues.length) {
        return null;
    }

    return (
        <div
            className={cn(
                'pointer-events-auto rounded-[22px] bg-card px-4 py-3',
                'max-h-[40vh] overflow-y-auto'
            )}
            style={{ boxShadow: 'var(--shadow-soft)' }}
        >
            <p className="text-[11px] font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                {workspaceCopy.issuesTitle}
            </p>
            <div className="mt-2 flex flex-col gap-1">
                {issues.map((issue, index) => {
                    const title = issueCodeLabel[issue.code] ?? issue.code;
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
                                'h-auto w-full justify-start rounded-2xl px-2 py-1.5',
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
                                {issue.message}
                            </span>
                        </Button>
                    );
                })}
            </div>
        </div>
    );
};
