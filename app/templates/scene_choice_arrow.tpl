{if not ch.is_editable}
    span.choice-label-text {ch.label}
{if ch.is_branch}
    span.choice-arrow
        svg.choice-branch-icon src: /icons/branch.svg
{else}
    span.choice-arrow {ch.arrow}
