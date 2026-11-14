article.scene-card .scene-focused?: {IsPresent} .scene-dimmed?: {!IsPresent} .has-illustration?: {HasImage} .scene-untitled?: {!HasTitle}
    {if HasTitle}
        .scene-header-block
            .scene-title-row
                {if IsPresent}
                    .scene-title-with-badge
                        span.present-scene-badge
                            svg.present-scene-icon src: /icons/asterisk.svg
                        h2.scene-title.scene-title-present {Name}
                {else}
                    h2.scene-title {Name}
                    .scene-contextual-rewind
                        button.contextual-rewind-btn
                            data-hint: "{t_rewind_scene_hint}"
                            svg.rewind-icon src: /icons/reset.svg

    {if HasImage}
        .scene-illustration-spacer
            img.scene-portrait-img
                loading: lazy
                decoding: async
                alt: "{RailTitle}"
                src: {ImageURL}

    .scene-body-block
        {if not IsPresent and not HasTitle}
            .scene-contextual-rewind.scene-contextual-rewind-floating
                button.contextual-rewind-btn
                    data-hint: "{t_rewind_scene_hint}"
                    svg.rewind-icon src: /icons/reset.svg
        .scene-prose .is-screenplay?: {IsScreenplayMode}
            {if IsScreenplayMode}
                {for blk in ScreenplayBlocks}
                    {if blk.Kind == "slugline"}
                        .sp-slugline
                            {if ShowSceneNum}
                                span.sp-scene-num "{SceneNum}."
                            span.sp-slugline-text {blk.Text}
                            {if ShowSceneNum}
                                span.sp-scene-num-right "{SceneNum}."
                    {elif blk.Kind == "character"}
                        .sp-character {blk.Text}
                    {elif blk.Kind == "parenthetical"}
                        .sp-parenthetical {blk.Text}
                    {elif blk.Kind == "dialogue"}
                        .sp-dialogue {blk.Text}
                    {elif blk.Kind == "transition"}
                        .sp-transition {blk.Text}
                    {else}
                        p.sp-action {blk.Text}
            {else}
                {prose Prose}
        {if IsPresent}
            .scene-choices-list
                {if IsTerminal}
                    button.choice-reset-btn
                        {if IsScreenplayMode}
                            span {t_end_screenplay_reset}
                        {else}
                            span {t_end_story_reset}
                        span.choice-arrow "↺"
                {else}
                    {for ch in Choices}
                        button.choice-btn .choice-locked?: {ch.ShowWhenLocked}
                            disabled?: {ch.ShowWhenLocked}
                            span.choice-label-text {ch.Label}
                            {if IsBranching and not ch.IsEnding}
                                span.choice-arrow
                                    svg.choice-branch-icon src: /icons/branch.svg
                            {elif ch.IsEnding}
                                span.choice-arrow {t_end_upper}
                            {else}
                                span.choice-arrow "»"
