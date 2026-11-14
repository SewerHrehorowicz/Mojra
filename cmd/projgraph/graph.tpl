doctype html
html lang: pl
 head
  meta charset: UTF-8
  meta name: viewport content: "width=device-width, initial-scale=1.0"
  title "Mojra — Project Graph ({Scope})"
  link rel: stylesheet href: graph.css
 body
  header.graph-header
   h1.graph-title "MOJRA PROJECT GRAPH"
   div.graph-meta "Scope: {Scope} | Dirs: {TotalDirs} | Files: {TotalFiles} | Symbols: {TotalSymbols}"
  main.graph-canvas
   {for dir in Dirs}
    details.node-branch.tree-dir open: open
     summary.node-card.dir-card "{dir.Path}"
     .node-children
      {for file in dir.Files}
       details.node-branch.tree-file open: open
        summary.node-card.file-card "{file.Name}"
        .node-children
         {for sym in file.Symbols}
          details.node-branch.tree-sym#{sym.ID} open: open
           summary.node-card.sym-card.kind-{sym.Kind} "[{sym.Kind}] {sym.Name} ({sym.LineRange})"
           .node-children.sym-leaves
            {for f in sym.Fields}
             div.leaf-node.sym-field ". {f}"
            {for edge in sym.OutEdges}
             a.leaf-node.edge-link.edge-out href: "#{edge.TargetID}" "→ {edge.Kind}: {edge.TargetLabel}"
            {for edge in sym.InEdges}
             a.leaf-node.edge-link.edge-in href: "#{edge.TargetID}" "← {edge.Kind}: {edge.TargetLabel}"
