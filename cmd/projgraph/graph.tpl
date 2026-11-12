doctype html
html lang: pl
 head
  meta charset: UTF-8
  meta name: viewport content: "width=device-width, initial-scale=1.0"
  title "Mojra — Project Graph ({scope})"
  link rel: stylesheet href: graph.css
 body
  header.graph-header
   h1.graph-title "MOJRA PROJECT GRAPH"
   div.graph-meta "Scope: {scope} | Dirs: {total_dirs} | Files: {total_files} | Symbols: {total_symbols}"
  main.graph-canvas
   {for dir in dirs}
    details.node-branch.tree-dir open: open
     summary.node-card.dir-card "{dir.path}"
     .node-children
      {for file in dir.files}
       details.node-branch.tree-file open: open
        summary.node-card.file-card "{file.name}"
        .node-children
         {for sym in file.symbols}
          details.node-branch.tree-sym#{sym.id} open: open
           summary.node-card.sym-card.kind-{sym.kind} "[{sym.kind}] {sym.name} ({sym.line_range})"
           .node-children.sym-leaves
            {for f in sym.fields}
             div.leaf-node.sym-field ". {f}"
            {for edge in sym.out_edges}
             a.leaf-node.edge-link.edge-out href: "#{edge.target_id}" "→ {edge.kind}: {edge.target_label}"
            {for edge in sym.in_edges}
             a.leaf-node.edge-link.edge-in href: "#{edge.target_id}" "← {edge.kind}: {edge.target_label}"
