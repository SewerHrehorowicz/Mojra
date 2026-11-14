{if not ch.IsEditable}
    span.choice-label-text {ch.Label}
{if ch.IsBranch}
    span.choice-arrow
        svg.choice-branch-icon src: /icons/branch.svg
{elif ch.IsEditable}
    span.choice-arrow "↺"
{elif ch.IsEnding}
    span.choice-arrow {t_end_upper}
{else}
    span.choice-arrow "»"
