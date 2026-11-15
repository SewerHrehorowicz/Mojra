.slugline-helper-pills
    {for p in SluglinePrefixPills}
        button.slugline-pill-btn
            data-scene: "{SelectedNode.ID}"
            data-prefix: "{p}"
            {p}
    {for t in SluglineTimePills}
        button.slugline-pill-btn
            data-scene: "{SelectedNode.ID}"
            data-time: "{t}"
            {t}
