(function () {
  var launchBtn = document.getElementById("btn-launch-wasm");
  var closeWasmBtn = document.getElementById("btn-close-wasm");
  var wasmOverlay = document.getElementById("wasm-overlay");

  var openBetaBtn = document.getElementById("btn-open-beta");
  var closeBetaBtn = document.getElementById("btn-close-beta");
  var betaOverlay = document.getElementById("beta-overlay");
  var betaForm = document.getElementById("beta-form");
  var betaEmail = document.getElementById("beta-email");
  var betaReason = document.getElementById("beta-reason");
  var betaSubmitLabel = document.getElementById("beta-submit-label");

  function openWasmStage() {
    document.body.classList.remove("beta-open");
    document.body.classList.add("wasm-open");
  }

  function closeWasmStage() {
    document.body.classList.remove("wasm-open");
    var storyAudio = document.getElementById("story-audio");
    if (storyAudio && typeof storyAudio.pause === "function") {
      storyAudio.pause();
    }
  }

  function openBetaModal() {
    document.body.classList.remove("wasm-open");
    document.body.classList.add("beta-open");
    if (betaEmail) {
      setTimeout(function () {
        betaEmail.focus();
      }, 60);
    }
  }

  function closeBetaModal() {
    document.body.classList.remove("beta-open");
  }

  if (launchBtn) launchBtn.addEventListener("click", openWasmStage);
  if (closeWasmBtn) closeWasmBtn.addEventListener("click", closeWasmStage);
  if (wasmOverlay) {
    wasmOverlay.addEventListener("click", function (e) {
      if (e.target === wasmOverlay) closeWasmStage();
    });
  }

  if (openBetaBtn) openBetaBtn.addEventListener("click", openBetaModal);
  if (closeBetaBtn) closeBetaBtn.addEventListener("click", closeBetaModal);
  if (betaOverlay) {
    betaOverlay.addEventListener("click", function (e) {
      if (e.target === betaOverlay) closeBetaModal();
    });
  }

  if (betaReason) {
    var syncTextareaHeight = function () {
      betaReason.style.height = "auto";
      betaReason.style.height = betaReason.scrollHeight + "px";
    };
    betaReason.addEventListener("input", syncTextareaHeight);
  }

  if (betaForm) {
    betaForm.addEventListener("submit", function (e) {
      e.preventDefault();
      var emailVal = betaEmail ? betaEmail.value.trim() : "";
      var reasonVal = betaReason ? betaReason.value.trim() : "";
      if (!emailVal || !reasonVal) return;

      var subject = encodeURIComponent("Mojra Beta — Zgłoszenie (" + emailVal + ")");
      var body = encodeURIComponent(
        "E-mail: " + emailVal + "\n\nCo tworzę i do czego użyję Mojry:\n" + reasonVal
      );
      window.location.href = "mailto:beta@mojra.pl?subject=" + subject + "&body=" + body;

      if (betaSubmitLabel) {
        betaSubmitLabel.textContent = "PRZYGOTOWANO ZGŁOSZENIE ✓";
      }
      setTimeout(function () {
        closeBetaModal();
        betaForm.reset();
        if (betaReason) betaReason.style.height = "auto";
        if (betaSubmitLabel) betaSubmitLabel.textContent = "WYŚLIJ ZGŁOSZENIE";
      }, 1100);
    });
  }

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      closeWasmStage();
      closeBetaModal();
    }
  });

  // =========================================================================
  // Structured Story Graph (Prolog -> Rozwidlenia -> Epilogi) with Gentle Motion
  // =========================================================================
  var canvas = document.getElementById("loom-canvas");
  if (!canvas) return;
  var ctx = canvas.getContext("2d");
  if (!ctx) return;

  var width = 0;
  var height = 0;
  var dpr = 1;
  var nodes = [];
  var edges = [];
  var pulses = [];
  var originNode = null;

  function rand(min, max) {
    return min + Math.random() * (max - min);
  }

  function buildStoryGraph() {
    nodes = [];
    edges = [];
    pulses = [];

    // 8-stage narrative DAG from left (1 origin) -> branching middle acts -> right (endings)
    // Center stages (cols 3, 4) route cleanly along upper and lower story arcs around the hero text
    var stageSpecs = [
      // Col 0: Prolog (single starting scene)
      { x: 0.05, ys: [0.50], role: "start" },
      // Col 1: Akt I — pierwsze rozwidlenie
      { x: 0.15, ys: [0.34, 0.66], role: "branch" },
      // Col 2: Akt II A — rozwinięcie wątków
      { x: 0.27, ys: [0.21, 0.39, 0.62, 0.79], role: "act" },
      // Col 3: Akt II B — górne i dolne pasmo wokół centrum
      { x: 0.41, ys: [0.16, 0.27, 0.74, 0.84], role: "act" },
      // Col 4: Punkt zwrotny — górne i dolne pasmo wokół centrum
      { x: 0.59, ys: [0.16, 0.27, 0.74, 0.84], role: "climax" },
      // Col 5: Akt III A — splatanie konsekwencji
      { x: 0.73, ys: [0.22, 0.40, 0.61, 0.78], role: "act" },
      // Col 6: Akt III B — finałowe ścieżki
      { x: 0.85, ys: [0.33, 0.50, 0.67], role: "branch" },
      // Col 7: Epilog (2 lub 3 zakończenia opowieści)
      { x: 0.94, ys: Math.random() < 0.5 ? [0.38, 0.62] : [0.30, 0.50, 0.70], role: "end" }
    ];

    var stages = [];
    var idCounter = 0;

    for (var s = 0; s < stageSpecs.length; s++) {
      var spec = stageSpecs[s];
      var colList = [];
      for (var i = 0; i < spec.ys.length; i++) {
        // Subtle procedural variation per load while keeping strict structural order
        var jitterX = s === 0 || s === stageSpecs.length - 1 ? 0 : rand(-0.014, 0.014);
        var jitterY = s === 0 ? rand(-0.02, 0.02) : rand(-0.022, 0.022);
        var nx = spec.x + jitterX;
        var ny = spec.ys[i] + jitterY;

        var doubleRing =
          spec.role === "start" ||
          spec.role === "end" ||
          (spec.role === "branch" && i === 0) ||
          (spec.role === "climax" && (i === 1 || i === 2));
        var hollow = !doubleRing && (i + s) % 2 === 1;

        var node = {
          id: idCounter++,
          col: s,
          row: i,
          anchorX: nx,
          anchorY: ny,
          // Gentle, low-amplitude, slow-period breathing (7-11px max, ~28-36s cycle)
          ampX: rand(5.5, 9.5),
          ampY: rand(6.5, 11.0),
          freqX: rand(0.00017, 0.00025),
          freqY: rand(0.00015, 0.00023),
          phaseX: s * 0.65 + i * 0.9 + rand(-0.2, 0.2),
          phaseY: s * 0.55 + i * 1.1 + rand(-0.2, 0.2),
          x: 0,
          y: 0,
          hollow: hollow,
          doubleRing: doubleRing,
          glow: 0,
          inCount: 0,
          outEdges: []
        };
        nodes.push(node);
        colList.push(node);
      }
      stages.push(colList);
    }

    originNode = stages[0][0];

    function linkNodes(a, b) {
      for (var k = 0; k < a.outEdges.length; k++) {
        if (a.outEdges[k].to === b) return;
      }
      var dy = b.anchorY - a.anchorY;
      var bend = dy * 0.18 + rand(-0.018, 0.018);
      var edge = {
        from: a,
        to: b,
        bend: bend
      };
      edges.push(edge);
      a.outEdges.push(edge);
      b.inCount++;
    }

    // Build clean forward-only story connections between consecutive acts
    for (var c = 0; c < stages.length - 1; c++) {
      var curr = stages[c];
      var next = stages[c + 1];

      // 1. Each node in curr connects to its nearest neighbor(s) in next within the same hemisphere
      for (var u = 0; u < curr.length; u++) {
        var src = curr[u];
        var candidates = next.slice().sort(function (n1, n2) {
          return Math.abs(n1.anchorY - src.anchorY) - Math.abs(n2.anchorY - src.anchorY);
        });

        // Always connect to closest forward node
        linkNodes(src, candidates[0]);

        // Branch to a second forward node if it doesn't cut across the central hero box
        if (candidates.length > 1) {
          var second = candidates[1];
          var crossesCenter =
            c >= 2 &&
            c <= 4 &&
            ((src.anchorY < 0.45 && second.anchorY > 0.55) ||
              (src.anchorY > 0.55 && second.anchorY < 0.45));
          if (!crossesCenter && (c <= 1 || Math.random() < 0.58)) {
            linkNodes(src, second);
          }
        }
      }

      // 2. Guarantee every node in next has at least one incoming story thread
      for (var v = 0; v < next.length; v++) {
        var dst = next[v];
        if (dst.inCount === 0) {
          var parents = curr.slice().sort(function (n1, n2) {
            return Math.abs(n1.anchorY - dst.anchorY) - Math.abs(n2.anchorY - dst.anchorY);
          });
          linkNodes(parents[0], dst);
        }
      }
    }
  }

  function resize() {
    dpr = Math.min(window.devicePixelRatio || 1, 2);
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = Math.floor(width * dpr);
    canvas.height = Math.floor(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  function updateNodePositions(t) {
    for (var i = 0; i < nodes.length; i++) {
      var n = nodes[i];
      // Nodes stay tethered to their structural anchor and sway gently in place
      var dx = Math.sin(t * n.freqX + n.phaseX) * n.ampX;
      var dy = Math.cos(t * n.freqY + n.phaseY) * n.ampY;
      n.x = n.anchorX * width + dx;
      n.y = n.anchorY * height + dy;

      if (n.glow > 0.001) {
        n.glow *= 0.95;
      } else {
        n.glow = 0;
      }
    }
  }

  function sampleEdgePoint(edge, u) {
    var x0 = edge.from.x;
    var y0 = edge.from.y;
    var x2 = edge.to.x;
    var y2 = edge.to.y;
    var mx = (x0 + x2) * 0.5;
    var my = (y0 + y2) * 0.5;
    var dx = x2 - x0;
    var dy = y2 - y0;
    var cx = mx - dy * edge.bend * 1.8;
    var cy = my + dx * edge.bend * 1.8;
    var inv = 1 - u;
    return {
      x: inv * inv * x0 + 2 * inv * u * cx + u * u * x2,
      y: inv * inv * y0 + 2 * inv * u * cy + u * u * y2,
      cx: cx,
      cy: cy
    };
  }

  function spawnPulseFromEdge(edge) {
    if (!edge) return;
    edge.from.glow = 0.9;
    pulses.push({
      edge: edge,
      u: 0,
      speed: 0.0036 + Math.random() * 0.0012,
      tail: 0.25
    });
  }

  function startNewStoryImpulse() {
    if (!originNode || originNode.outEdges.length === 0) return;
    originNode.glow = 1.0;
    var firstEdge =
      originNode.outEdges[Math.floor(Math.random() * originNode.outEdges.length)];
    spawnPulseFromEdge(firstEdge);
  }

  buildStoryGraph();
  resize();
  window.addEventListener("resize", resize);

  startNewStoryImpulse();

  var lastStoryStart = performance.now();

  function frame(now) {
    updateNodePositions(now);
    ctx.clearRect(0, 0, width, height);

    if (
      (pulses.length === 0 && now - lastStoryStart > 900) ||
      (now - lastStoryStart > 3600 && pulses.length < 3)
    ) {
      startNewStoryImpulse();
      lastStoryStart = now;
    }

    // 1. Draw structured narrative threads
    ctx.lineWidth = 1;
    ctx.strokeStyle = "rgba(255, 255, 255, 0.075)";
    for (var i = 0; i < edges.length; i++) {
      var e = edges[i];
      var mid = sampleEdgePoint(e, 0.5);
      ctx.beginPath();
      ctx.moveTo(e.from.x, e.from.y);
      ctx.quadraticCurveTo(mid.cx, mid.cy, e.to.x, e.to.y);
      ctx.stroke();
    }

    // 2. Draw flowing story impulses from Prolog -> Branches -> Epilog
    for (var pIdx = pulses.length - 1; pIdx >= 0; pIdx--) {
      var p = pulses[pIdx];
      p.u += p.speed;

      var steps = 14;
      for (var s = 0; s < steps; s++) {
        var uA = p.u - (s / steps) * p.tail;
        var uB = p.u - ((s + 1) / steps) * p.tail;
        if (uA <= 0) continue;
        var clampedA = Math.min(1, Math.max(0, uA));
        var clampedB = Math.min(1, Math.max(0, uB));
        if (clampedA <= clampedB) continue;

        var ptA = sampleEdgePoint(p.edge, clampedA);
        var ptB = sampleEdgePoint(p.edge, clampedB);
        var alpha = (1 - s / steps) * 0.42;
        if (p.u > 1) {
          alpha *= Math.max(0, 1 - (p.u - 1) / p.tail);
        }

        ctx.strokeStyle = "rgba(255, 255, 255, " + alpha.toFixed(3) + ")";
        ctx.lineWidth = 1.35;
        ctx.beginPath();
        ctx.moveTo(ptB.x, ptB.y);
        ctx.lineTo(ptA.x, ptA.y);
        ctx.stroke();
      }

      if (p.u <= 1) {
        var head = sampleEdgePoint(p.edge, p.u);
        ctx.fillStyle = "rgba(255, 255, 255, 0.62)";
        ctx.beginPath();
        ctx.arc(head.x, head.y, 1.8, 0, Math.PI * 2);
        ctx.fill();
      }

      if (p.u >= 1 && !p.branched) {
        p.branched = true;
        var targetNode = p.edge.to;
        targetNode.glow = 1.0;
        if (targetNode.outEdges.length > 0 && pulses.length < 6) {
          var chosen =
            targetNode.outEdges[Math.floor(Math.random() * targetNode.outEdges.length)];
          spawnPulseFromEdge(chosen);
          // At key branching nodes, occasionally split into two parallel threads
          if (targetNode.outEdges.length > 1 && Math.random() < 0.28 && pulses.length < 5) {
            for (var k = 0; k < targetNode.outEdges.length; k++) {
              if (targetNode.outEdges[k] !== chosen) {
                spawnPulseFromEdge(targetNode.outEdges[k]);
                break;
              }
            }
          }
        }
      }

      if (p.u > 1 + p.tail) {
        pulses.splice(pIdx, 1);
      }
    }

    // 3. Draw Mojra story nodes
    for (var nIdx = 0; nIdx < nodes.length; nIdx++) {
      var n = nodes[nIdx];
      var baseAlpha = 0.16 + n.glow * 0.52;
      if (n.doubleRing) {
        ctx.fillStyle = "#000000";
        ctx.beginPath();
        ctx.arc(n.x, n.y, 4.2, 0, Math.PI * 2);
        ctx.fill();
        ctx.strokeStyle = "rgba(255, 255, 255, " + baseAlpha.toFixed(3) + ")";
        ctx.lineWidth = 1.15;
        ctx.stroke();

        ctx.fillStyle = "rgba(255, 255, 255, " + (0.24 + n.glow * 0.55).toFixed(3) + ")";
        ctx.beginPath();
        ctx.arc(n.x, n.y, 1.5, 0, Math.PI * 2);
        ctx.fill();
      } else if (n.hollow) {
        ctx.fillStyle = "#000000";
        ctx.beginPath();
        ctx.arc(n.x, n.y, 2.7, 0, Math.PI * 2);
        ctx.fill();
        ctx.strokeStyle = "rgba(255, 255, 255, " + baseAlpha.toFixed(3) + ")";
        ctx.lineWidth = 1.1;
        ctx.stroke();
      } else {
        ctx.fillStyle = "rgba(255, 255, 255, " + baseAlpha.toFixed(3) + ")";
        ctx.beginPath();
        ctx.arc(n.x, n.y, 2.4, 0, Math.PI * 2);
        ctx.fill();
      }
    }

    requestAnimationFrame(frame);
  }

  requestAnimationFrame(frame);
})();
