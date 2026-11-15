(function () {
  var landingConfig = {
    ids: {
      launchWasm: "btn-launch-wasm",
      closeWasm: "btn-close-wasm",
      wasmOverlay: "wasm-overlay",
      openBeta: "btn-open-beta",
      closeBeta: "btn-close-beta",
      betaOverlay: "beta-overlay",
      betaForm: "beta-form",
      betaEmail: "beta-email",
      betaReason: "beta-reason",
      betaSubmitLabel: "beta-submit-label",
      storyAudio: "story-audio",
      loomCanvas: "loom-canvas"
    },
    classes: {
      betaOpen: "beta-open",
      wasmOpen: "wasm-open"
    },
    events: {
      click: "click",
      input: "input",
      submit: "submit",
      keydown: "keydown",
      resize: "resize"
    },
    keys: {
      escape: "Escape"
    },
    roles: {
      start: "start",
      branch: "branch",
      act: "act",
      climax: "climax",
      end: "end"
    },
    styles: {
      heightAuto: "auto",
      pxSuffix: "px",
      ctx2d: "2d",
      fnType: "function",
      blackHex: "#000000",
      threadStroke: "rgba(255, 255, 255, 0.075)",
      pulseHeadFill: "rgba(255, 255, 255, 0.62)",
      whiteRgbaPrefix: "rgba(255, 255, 255, ",
      rgbaSuffix: ")"
    },
    beta: {
      subjectPrefix: "Mojra Beta — Zgłoszenie (",
      subjectSuffix: ")",
      bodyEmailPrefix: "E-mail: ",
      bodyReasonHeader: "\n\nCo tworzę i do czego użyję Mojry:\n",
      mailtoPrefix: "mailto:beta@mojra.pl?subject=",
      mailtoBodySep: "&body=",
      submittedLabel: "PRZYGOTOWANO ZGŁOSZENIE ✓",
      submitLabel: "WYŚLIJ ZGŁOSZENIE",
      focusDelayMs: 60,
      resetDelayMs: 1100
    }
  };

  var launchBtn = document.getElementById(landingConfig.ids.launchWasm);
  var closeWasmBtn = document.getElementById(landingConfig.ids.closeWasm);
  var wasmOverlay = document.getElementById(landingConfig.ids.wasmOverlay);

  var openBetaBtn = document.getElementById(landingConfig.ids.openBeta);
  var closeBetaBtn = document.getElementById(landingConfig.ids.closeBeta);
  var betaOverlay = document.getElementById(landingConfig.ids.betaOverlay);
  var betaForm = document.getElementById(landingConfig.ids.betaForm);
  var betaEmail = document.getElementById(landingConfig.ids.betaEmail);
  var betaReason = document.getElementById(landingConfig.ids.betaReason);
  var betaSubmitLabel = document.getElementById(landingConfig.ids.betaSubmitLabel);

  function openWasmStage() {
    document.body.classList.remove(landingConfig.classes.betaOpen);
    document.body.classList.add(landingConfig.classes.wasmOpen);
  }

  function closeWasmStage() {
    document.body.classList.remove(landingConfig.classes.wasmOpen);
    var storyAudio = document.getElementById(landingConfig.ids.storyAudio);
    if (storyAudio && typeof storyAudio.pause === landingConfig.styles.fnType) {
      storyAudio.pause();
    }
  }

  function openBetaModal() {
    document.body.classList.remove(landingConfig.classes.wasmOpen);
    document.body.classList.add(landingConfig.classes.betaOpen);
    if (betaEmail) {
      setTimeout(function () {
        betaEmail.focus();
      }, landingConfig.beta.focusDelayMs);
    }
  }

  function closeBetaModal() {
    document.body.classList.remove(landingConfig.classes.betaOpen);
  }

  if (launchBtn) launchBtn.addEventListener(landingConfig.events.click, openWasmStage);
  if (closeWasmBtn) closeWasmBtn.addEventListener(landingConfig.events.click, closeWasmStage);
  if (wasmOverlay) {
    wasmOverlay.addEventListener(landingConfig.events.click, function (evt) {
      if (evt.target === wasmOverlay) closeWasmStage();
    });
  }

  if (openBetaBtn) openBetaBtn.addEventListener(landingConfig.events.click, openBetaModal);
  if (closeBetaBtn) closeBetaBtn.addEventListener(landingConfig.events.click, closeBetaModal);
  if (betaOverlay) {
    betaOverlay.addEventListener(landingConfig.events.click, function (evt) {
      if (evt.target === betaOverlay) closeBetaModal();
    });
  }

  if (betaReason) {
    var syncTextareaHeight = function () {
      betaReason.style.height = landingConfig.styles.heightAuto;
      betaReason.style.height = betaReason.scrollHeight + landingConfig.styles.pxSuffix;
    };
    betaReason.addEventListener(landingConfig.events.input, syncTextareaHeight);
  }

  if (betaForm) {
    betaForm.addEventListener(landingConfig.events.submit, function (evt) {
      evt.preventDefault();
      var emailVal = betaEmail ? betaEmail.value.trim() : "";
      var reasonVal = betaReason ? betaReason.value.trim() : "";
      if (!emailVal || !reasonVal) return;

      var subject = encodeURIComponent(landingConfig.beta.subjectPrefix + emailVal + landingConfig.beta.subjectSuffix);
      var body = encodeURIComponent(
        landingConfig.beta.bodyEmailPrefix + emailVal + landingConfig.beta.bodyReasonHeader + reasonVal
      );
      window.location.href = landingConfig.beta.mailtoPrefix + subject + landingConfig.beta.mailtoBodySep + body;

      if (betaSubmitLabel) {
        betaSubmitLabel.textContent = landingConfig.beta.submittedLabel;
      }
      setTimeout(function () {
        closeBetaModal();
        betaForm.reset();
        if (betaReason) betaReason.style.height = landingConfig.styles.heightAuto;
        if (betaSubmitLabel) betaSubmitLabel.textContent = landingConfig.beta.submitLabel;
      }, landingConfig.beta.resetDelayMs);
    });
  }

  document.addEventListener(landingConfig.events.keydown, function (evt) {
    if (evt.key === landingConfig.keys.escape) {
      closeWasmStage();
      closeBetaModal();
    }
  });

  var canvas = document.getElementById(landingConfig.ids.loomCanvas);
  if (!canvas) return;
  var ctx = canvas.getContext(landingConfig.styles.ctx2d);
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

    var roles = landingConfig.roles;
    var stageSpecs = [
      { x: 0.05, ys: [0.50], role: roles.start },
      { x: 0.15, ys: [0.34, 0.66], role: roles.branch },
      { x: 0.27, ys: [0.21, 0.39, 0.62, 0.79], role: roles.act },
      { x: 0.41, ys: [0.16, 0.27, 0.74, 0.84], role: roles.act },
      { x: 0.59, ys: [0.16, 0.27, 0.74, 0.84], role: roles.climax },
      { x: 0.73, ys: [0.22, 0.40, 0.61, 0.78], role: roles.act },
      { x: 0.85, ys: [0.33, 0.50, 0.67], role: roles.branch },
      { x: 0.94, ys: Math.random() < 0.5 ? [0.38, 0.62] : [0.30, 0.50, 0.70], role: roles.end }
    ];

    var stages = [];
    var idCounter = 0;

    for (var s = 0; s < stageSpecs.length; s++) {
      var spec = stageSpecs[s];
      var colList = [];
      for (var i = 0; i < spec.ys.length; i++) {
        var jitterX = s === 0 || s === stageSpecs.length - 1 ? 0 : rand(-0.014, 0.014);
        var jitterY = s === 0 ? rand(-0.02, 0.02) : rand(-0.022, 0.022);
        var nx = spec.x + jitterX;
        var ny = spec.ys[i] + jitterY;

        var doubleRing =
          spec.role === roles.start ||
          spec.role === roles.end ||
          (spec.role === roles.branch && i === 0) ||
          (spec.role === roles.climax && (i === 1 || i === 2));
        var hollow = !doubleRing && (i + s) % 2 === 1;

        var node = {
          id: idCounter++,
          col: s,
          row: i,
          anchorX: nx,
          anchorY: ny,
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

    function linkNodes(fromNode, toNode) {
      for (var k = 0; k < fromNode.outEdges.length; k++) {
        if (fromNode.outEdges[k].to === toNode) return;
      }
      var dy = toNode.anchorY - fromNode.anchorY;
      var bend = dy * 0.18 + rand(-0.018, 0.018);
      var edge = {
        from: fromNode,
        to: toNode,
        bend: bend
      };
      edges.push(edge);
      fromNode.outEdges.push(edge);
      toNode.inCount++;
    }

    for (var c = 0; c < stages.length - 1; c++) {
      var curr = stages[c];
      var next = stages[c + 1];

      for (var u = 0; u < curr.length; u++) {
        var src = curr[u];
        var candidates = next.slice().sort(function (n1, n2) {
          return Math.abs(n1.anchorY - src.anchorY) - Math.abs(n2.anchorY - src.anchorY);
        });

        linkNodes(src, candidates[0]);

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

  function updateNodePositions(timeMs) {
    for (var i = 0; i < nodes.length; i++) {
      var n = nodes[i];
      var dx = Math.sin(timeMs * n.freqX + n.phaseX) * n.ampX;
      var dy = Math.cos(timeMs * n.freqY + n.phaseY) * n.ampY;
      n.x = n.anchorX * width + dx;
      n.y = n.anchorY * height + dy;

      if (n.glow > 0.001) {
        n.glow *= 0.95;
      } else {
        n.glow = 0;
      }
    }
  }

  function sampleEdgePoint(edge, ratio) {
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
    var inv = 1 - ratio;
    return {
      x: inv * inv * x0 + 2 * inv * ratio * cx + ratio * ratio * x2,
      y: inv * inv * y0 + 2 * inv * ratio * cy + ratio * ratio * y2,
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
  window.addEventListener(landingConfig.events.resize, resize);

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

    var st = landingConfig.styles;
    ctx.lineWidth = 1;
    ctx.strokeStyle = st.threadStroke;
    for (var i = 0; i < edges.length; i++) {
      var edgeItem = edges[i];
      var mid = sampleEdgePoint(edgeItem, 0.5);
      ctx.beginPath();
      ctx.moveTo(edgeItem.from.x, edgeItem.from.y);
      ctx.quadraticCurveTo(mid.cx, mid.cy, edgeItem.to.x, edgeItem.to.y);
      ctx.stroke();
    }

    for (var pIdx = pulses.length - 1; pIdx >= 0; pIdx--) {
      var pulse = pulses[pIdx];
      pulse.u += pulse.speed;

      var steps = 14;
      for (var s = 0; s < steps; s++) {
        var uA = pulse.u - (s / steps) * pulse.tail;
        var uB = pulse.u - ((s + 1) / steps) * pulse.tail;
        if (uA <= 0) continue;
        var clampedA = Math.min(1, Math.max(0, uA));
        var clampedB = Math.min(1, Math.max(0, uB));
        if (clampedA <= clampedB) continue;

        var ptA = sampleEdgePoint(pulse.edge, clampedA);
        var ptB = sampleEdgePoint(pulse.edge, clampedB);
        var alpha = (1 - s / steps) * 0.42;
        if (pulse.u > 1) {
          alpha *= Math.max(0, 1 - (pulse.u - 1) / pulse.tail);
        }

        ctx.strokeStyle = st.whiteRgbaPrefix + alpha.toFixed(3) + st.rgbaSuffix;
        ctx.lineWidth = 1.35;
        ctx.beginPath();
        ctx.moveTo(ptB.x, ptB.y);
        ctx.lineTo(ptA.x, ptA.y);
        ctx.stroke();
      }

      if (pulse.u <= 1) {
        var head = sampleEdgePoint(pulse.edge, pulse.u);
        ctx.fillStyle = st.pulseHeadFill;
        ctx.beginPath();
        ctx.arc(head.x, head.y, 1.8, 0, Math.PI * 2);
        ctx.fill();
      }

      if (pulse.u >= 1 && !pulse.branched) {
        pulse.branched = true;
        var targetNode = pulse.edge.to;
        targetNode.glow = 1.0;
        if (targetNode.outEdges.length > 0 && pulses.length < 6) {
          var chosen =
            targetNode.outEdges[Math.floor(Math.random() * targetNode.outEdges.length)];
          spawnPulseFromEdge(chosen);
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

      if (pulse.u > 1 + pulse.tail) {
        pulses.splice(pIdx, 1);
      }
    }

    for (var nIdx = 0; nIdx < nodes.length; nIdx++) {
      var nodeItem = nodes[nIdx];
      var baseAlpha = 0.16 + nodeItem.glow * 0.52;
      if (nodeItem.doubleRing) {
        ctx.fillStyle = st.blackHex;
        ctx.beginPath();
        ctx.arc(nodeItem.x, nodeItem.y, 4.2, 0, Math.PI * 2);
        ctx.fill();
        ctx.strokeStyle = st.whiteRgbaPrefix + baseAlpha.toFixed(3) + st.rgbaSuffix;
        ctx.lineWidth = 1.15;
        ctx.stroke();

        ctx.fillStyle = st.whiteRgbaPrefix + (0.24 + nodeItem.glow * 0.55).toFixed(3) + st.rgbaSuffix;
        ctx.beginPath();
        ctx.arc(nodeItem.x, nodeItem.y, 1.5, 0, Math.PI * 2);
        ctx.fill();
      } else if (nodeItem.hollow) {
        ctx.fillStyle = st.blackHex;
        ctx.beginPath();
        ctx.arc(nodeItem.x, nodeItem.y, 2.7, 0, Math.PI * 2);
        ctx.fill();
        ctx.strokeStyle = st.whiteRgbaPrefix + baseAlpha.toFixed(3) + st.rgbaSuffix;
        ctx.lineWidth = 1.1;
        ctx.stroke();
      } else {
        ctx.fillStyle = st.whiteRgbaPrefix + baseAlpha.toFixed(3) + st.rgbaSuffix;
        ctx.beginPath();
        ctx.arc(nodeItem.x, nodeItem.y, 2.4, 0, Math.PI * 2);
        ctx.fill();
      }
    }

    requestAnimationFrame(frame);
  }

  requestAnimationFrame(frame);
})();
