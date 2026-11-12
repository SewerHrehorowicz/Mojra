{if not ch.is_editable}
    span.choice-label-text {ch.label}
{if ch.is_branch}
    span.choice-arrow
        svg.choice-branch-icon src: /icons/branch.svg
{elif ch.is_editable}
    span.choice-arrow "↺"
{elif ch.is_ending}
    span.choice-arrow {t_end_upper}
{else}
    span.choice-arrow "»"
